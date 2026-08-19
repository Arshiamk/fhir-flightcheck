package controlplane

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type PostgresRepository struct {
	pool           *pgxpool.Pool
	organizationID string
}

func NewPostgresRepository(ctx context.Context, databaseURL, organizationID string) (*PostgresRepository, error) {
	if databaseURL == "" {
		return nil, errors.New("database URL is required")
	}
	if organizationID == "" {
		return nil, errors.New("organization ID is required")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	repository := &PostgresRepository{pool: pool, organizationID: organizationID}
	if err := repository.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := repository.Migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return repository, nil
}

func (p *PostgresRepository) Close() { p.pool.Close() }

func (p *PostgresRepository) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *PostgresRepository) Migrate(ctx context.Context) error {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fhir-flightcheck-migrations'))`); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("bootstrap migrations: %w", err)
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for index, entry := range entries {
		version := int64(index + 1)
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("read migration version: %w", err)
		}
		if applied {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version); err != nil {
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func (p *PostgresRepository) begin(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	tx, err := p.pool.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('flightcheck.organization_id', $1, true)`, p.organizationID); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	return tx, nil
}

func (p *PostgresRepository) reserve(ctx context.Context, tx pgx.Tx, idem Idempotency, resourceID string) (string, bool, error) {
	result, err := tx.Exec(ctx, `INSERT INTO idempotency_keys
		(organization_id,operation,key,request_sha256,resource_id) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT DO NOTHING`, p.organizationID, idem.Operation, idem.Key, idem.RequestHash, resourceID)
	if err != nil {
		return "", false, err
	}
	if result.RowsAffected() == 1 {
		return resourceID, false, nil
	}
	var requestHash, existingID string
	err = tx.QueryRow(ctx, `SELECT request_sha256,resource_id FROM idempotency_keys
		WHERE organization_id=$1 AND operation=$2 AND key=$3`,
		p.organizationID, idem.Operation, idem.Key).Scan(&requestHash, &existingID)
	if err != nil {
		return "", false, err
	}
	if requestHash != idem.RequestHash {
		return "", false, ErrIdempotencyConflict
	}
	return existingID, true, nil
}

func marshalBody(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal repository value: %w", err)
	}
	return body, nil
}

func scanJSON[T any](row pgx.Row) (T, error) {
	var zero T
	var body []byte
	if err := row.Scan(&body); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, ErrNotFound
		}
		return zero, err
	}
	if err := json.Unmarshal(body, &zero); err != nil {
		return zero, err
	}
	return zero, nil
}

func (p *PostgresRepository) CreateProject(ctx context.Context, value Project, idem Idempotency) (Project, bool, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Project{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	id, replay, err := p.reserve(ctx, tx, idem, value.ID)
	if err != nil {
		return Project{}, false, err
	}
	if replay {
		existing, err := scanJSON[Project](tx.QueryRow(ctx, `SELECT body FROM projects WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
		if err != nil {
			return Project{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	body, err := marshalBody(value)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO projects(organization_id,id,body,created_at) VALUES($1,$2,$3,$4)`,
			p.organizationID, value.ID, body, value.CreatedAt)
	}
	if err != nil {
		return Project{}, false, err
	}
	return value, false, tx.Commit(ctx)
}

func (p *PostgresRepository) GetProject(ctx context.Context, id string) (Project, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Project{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[Project](tx.QueryRow(ctx, `SELECT body FROM projects WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
	if err != nil {
		return Project{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) CreateTarget(ctx context.Context, value Target, idem Idempotency) (Target, bool, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Target{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	id, replay, err := p.reserve(ctx, tx, idem, value.ID)
	if err != nil {
		return Target{}, false, err
	}
	if replay {
		existing, err := scanJSON[Target](tx.QueryRow(ctx, `SELECT body FROM targets WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
		if err != nil {
			return Target{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	body, err := marshalBody(value)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO targets(organization_id,id,project_id,body,created_at) VALUES($1,$2,$3,$4,$5)`,
			p.organizationID, value.ID, value.ProjectID, body, value.CreatedAt)
	}
	if err != nil {
		return Target{}, false, err
	}
	return value, false, tx.Commit(ctx)
}

func (p *PostgresRepository) GetTarget(ctx context.Context, projectID, id string) (Target, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Target{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[Target](tx.QueryRow(ctx, `SELECT body FROM targets
		WHERE organization_id=$1 AND project_id=$2 AND id=$3`, p.organizationID, projectID, id))
	if err != nil {
		return Target{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) CreateQueuedRun(ctx context.Context, run Run, job JobRecord, outbox OutboxMessage, idem Idempotency) (Run, bool, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Run{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	id, replay, err := p.reserve(ctx, tx, idem, run.ID)
	if err != nil {
		return Run{}, false, err
	}
	if replay {
		existing, err := scanJSON[Run](tx.QueryRow(ctx, `SELECT body FROM runs WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
		if err != nil {
			return Run{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	runBody, err := marshalBody(run)
	if err != nil {
		return Run{}, false, err
	}
	jobBody, err := marshalBody(job)
	if err != nil {
		return Run{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO runs(organization_id,id,project_id,target_id,status,body,created_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, p.organizationID, run.ID, run.ProjectID, run.TargetID, run.Status, runBody, run.CreatedAt); err != nil {
		return Run{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO jobs(organization_id,id,run_id,status,body,created_at)
		VALUES($1,$2,$3,$4,$5,$6)`, p.organizationID, job.Job.JobID, run.ID, job.Status, jobBody, job.CreatedAt); err != nil {
		return Run{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO outbox(organization_id,id,subject,payload,available_at)
		VALUES($1,$2,$3,$4,$5)`, p.organizationID, outbox.ID, outbox.Subject, outbox.Payload, outbox.AvailableAt); err != nil {
		return Run{}, false, err
	}
	return run, false, tx.Commit(ctx)
}

func (p *PostgresRepository) GetRun(ctx context.Context, id string) (Run, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Run{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[Run](tx.QueryRow(ctx, `SELECT body FROM runs WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
	if err != nil {
		return Run{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) GetJob(ctx context.Context, id string) (JobRecord, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return JobRecord{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[JobRecord](tx.QueryRow(ctx, `SELECT body FROM jobs WHERE organization_id=$1 AND id=$2`, p.organizationID, id))
	if err != nil {
		return JobRecord{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) CompleteJob(ctx context.Context, jobID, completionHash string, findings []Finding, evidence []Evidence, report Report, completedAt time.Time) (Report, bool, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Report{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var jobBody []byte
	var status, existingHash, runID string
	err = tx.QueryRow(ctx, `SELECT body,status,COALESCE(completion_hash,''),run_id FROM jobs
		WHERE organization_id=$1 AND id=$2 FOR UPDATE`, p.organizationID, jobID).
		Scan(&jobBody, &status, &existingHash, &runID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, false, ErrNotFound
	}
	if err != nil {
		return Report{}, false, err
	}
	if status == "completed" {
		if existingHash != completionHash {
			return Report{}, false, ErrStaleCompletion
		}
		existing, err := scanJSON[Report](tx.QueryRow(ctx, `SELECT body FROM reports WHERE organization_id=$1 AND run_id=$2`, p.organizationID, runID))
		if err != nil {
			return Report{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	var runBody []byte
	var runStatus string
	if err := tx.QueryRow(ctx, `SELECT body,status FROM runs WHERE organization_id=$1 AND id=$2 FOR UPDATE`,
		p.organizationID, runID).Scan(&runBody, &runStatus); err != nil {
		return Report{}, false, err
	}
	if runStatus == "cancelled" {
		return Report{}, false, ErrCancelledRun
	}
	if status != "queued" && status != "dispatched" {
		return Report{}, false, ErrStaleCompletion
	}
	for _, finding := range findings {
		body, err := marshalBody(finding)
		if err != nil {
			return Report{}, false, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO findings(organization_id,finding_id,run_id,rule_id,body)
			VALUES($1,$2,$3,$4,$5)`, p.organizationID, finding.FindingID, runID, finding.RuleID, body); err != nil {
			return Report{}, false, err
		}
	}
	for _, item := range evidence {
		body, err := marshalBody(item)
		if err != nil {
			return Report{}, false, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO evidence(organization_id,evidence_id,run_id,body)
			VALUES($1,$2,$3,$4)`, p.organizationID, item.EvidenceID, runID, body); err != nil {
			return Report{}, false, err
		}
	}
	reportBody, err := marshalBody(report)
	if err != nil {
		return Report{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO reports(organization_id,report_id,run_id,body,created_at)
		VALUES($1,$2,$3,$4,$5)`, p.organizationID, report.ReportID, runID, reportBody, report.CreatedAt); err != nil {
		return Report{}, false, err
	}
	var job JobRecord
	if err := json.Unmarshal(jobBody, &job); err != nil {
		return Report{}, false, err
	}
	job.Status, job.CompletionHash, job.CompletedAt = "completed", completionHash, &completedAt
	jobBody, _ = marshalBody(job)
	if _, err := tx.Exec(ctx, `UPDATE jobs SET status='completed',completion_hash=$3,completed_at=$4,body=$5
		WHERE organization_id=$1 AND id=$2`, p.organizationID, jobID, completionHash, completedAt, jobBody); err != nil {
		return Report{}, false, err
	}
	var run Run
	if err := json.Unmarshal(runBody, &run); err != nil {
		return Report{}, false, err
	}
	run.Status, run.ReportID, run.CompletedAt = "completed", report.ReportID, &completedAt
	runBody, _ = marshalBody(run)
	if _, err := tx.Exec(ctx, `UPDATE runs SET status='completed',body=$3 WHERE organization_id=$1 AND id=$2`,
		p.organizationID, runID, runBody); err != nil {
		return Report{}, false, err
	}
	return report, false, tx.Commit(ctx)
}

func (p *PostgresRepository) GetReportByRun(ctx context.Context, runID string) (Report, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[Report](tx.QueryRow(ctx, `SELECT body FROM reports WHERE organization_id=$1 AND run_id=$2`, p.organizationID, runID))
	if err != nil {
		return Report{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) SetBaseline(ctx context.Context, value Baseline, idem Idempotency) (Baseline, bool, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Baseline{}, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	projectID, replay, err := p.reserve(ctx, tx, idem, value.ProjectID)
	if err != nil {
		return Baseline{}, false, err
	}
	if replay {
		existing, err := scanJSON[Baseline](tx.QueryRow(ctx, `SELECT body FROM baselines WHERE organization_id=$1 AND project_id=$2`, p.organizationID, projectID))
		if err != nil {
			return Baseline{}, false, err
		}
		return existing, true, tx.Commit(ctx)
	}
	body, err := marshalBody(value)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO baselines(organization_id,project_id,run_id,report_id,body)
			VALUES($1,$2,$3,$4,$5)
			ON CONFLICT (organization_id,project_id) DO UPDATE SET run_id=EXCLUDED.run_id,report_id=EXCLUDED.report_id,body=EXCLUDED.body`,
			p.organizationID, value.ProjectID, value.RunID, value.ReportID, body)
	}
	if err != nil {
		return Baseline{}, false, err
	}
	return value, false, tx.Commit(ctx)
}

func (p *PostgresRepository) GetBaseline(ctx context.Context, projectID string) (Baseline, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return Baseline{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	value, err := scanJSON[Baseline](tx.QueryRow(ctx, `SELECT body FROM baselines WHERE organization_id=$1 AND project_id=$2`, p.organizationID, projectID))
	if err != nil {
		return Baseline{}, err
	}
	return value, tx.Commit(ctx)
}

func (p *PostgresRepository) ClaimOutbox(ctx context.Context, limit int, lease time.Duration) ([]OutboxMessage, error) {
	tx, err := p.begin(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	rows, err := tx.Query(ctx, `WITH claimed AS (
		SELECT organization_id,id FROM outbox
		WHERE organization_id=$1 AND published_at IS NULL AND available_at<=now()
		  AND (lease_until IS NULL OR lease_until<now())
		ORDER BY available_at FOR UPDATE SKIP LOCKED LIMIT $2
	)
	UPDATE outbox o SET lease_until=now()+$3::interval,attempts=o.attempts+1
	FROM claimed c WHERE o.organization_id=c.organization_id AND o.id=c.id
	RETURNING o.id,o.organization_id,o.subject,o.payload,o.attempts,o.available_at`,
		p.organizationID, limit, lease.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []OutboxMessage
	for rows.Next() {
		var value OutboxMessage
		if err := rows.Scan(&value.ID, &value.OrganizationID, &value.Subject, &value.Payload, &value.Attempts, &value.AvailableAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return values, tx.Commit(ctx)
}

func (p *PostgresRepository) MarkOutboxPublished(ctx context.Context, id string) error {
	tx, err := p.begin(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `UPDATE outbox SET published_at=now(),lease_until=NULL
		WHERE organization_id=$1 AND id=$2 AND published_at IS NULL`, p.organizationID, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

func (p *PostgresRepository) MarkOutboxFailed(ctx context.Context, id string, availableAt time.Time) error {
	tx, err := p.begin(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	result, err := tx.Exec(ctx, `UPDATE outbox SET available_at=$3,lease_until=NULL,last_error_at=now()
		WHERE organization_id=$1 AND id=$2 AND published_at IS NULL`, p.organizationID, id, availableAt)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
