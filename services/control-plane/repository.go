package controlplane

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotFound            = errors.New("resource not found")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different request")
	ErrStaleCompletion     = errors.New("stale or conflicting completion")
	ErrCancelledRun        = errors.New("run is cancelled")
)

type Idempotency struct {
	Operation   string
	Key         string
	RequestHash string
}

// Repository is the persistence boundary. Implementations must make each
// Create/Set operation and its idempotency record atomic.
type Repository interface {
	CreateProject(context.Context, Project, Idempotency) (Project, bool, error)
	GetProject(context.Context, string) (Project, error)
	CreateTarget(context.Context, Target, Idempotency) (Target, bool, error)
	GetTarget(context.Context, string, string) (Target, error)
	CreateQueuedRun(context.Context, Run, JobRecord, OutboxMessage, Idempotency) (Run, bool, error)
	GetRun(context.Context, string) (Run, error)
	GetJob(context.Context, string) (JobRecord, error)
	CompleteJob(context.Context, string, string, []Finding, []Evidence, Report, time.Time) (Report, bool, error)
	GetReportByRun(context.Context, string) (Report, error)
	SetBaseline(context.Context, Baseline, Idempotency) (Baseline, bool, error)
	GetBaseline(context.Context, string) (Baseline, error)
	ClaimOutbox(context.Context, int, time.Duration) ([]OutboxMessage, error)
	MarkOutboxPublished(context.Context, string) error
	MarkOutboxFailed(context.Context, string, time.Time) error
	Ping(context.Context) error
}

type idempotencyRecord struct {
	requestHash string
	resourceID  string
}

type MemoryRepository struct {
	mu          sync.RWMutex
	projects    map[string]Project
	targets     map[string]Target
	runs        map[string]Run
	jobs        map[string]JobRecord
	reports     map[string]Report
	findings    map[string][]Finding
	evidence    map[string][]Evidence
	baselines   map[string]Baseline
	outbox      map[string]OutboxMessage
	outboxLease map[string]time.Time
	idempotency map[string]idempotencyRecord
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		projects: make(map[string]Project), targets: make(map[string]Target),
		runs: make(map[string]Run), jobs: make(map[string]JobRecord), reports: make(map[string]Report),
		findings: make(map[string][]Finding), evidence: make(map[string][]Evidence),
		baselines: make(map[string]Baseline), outbox: make(map[string]OutboxMessage),
		outboxLease: make(map[string]time.Time), idempotency: make(map[string]idempotencyRecord),
	}
}

func (m *MemoryRepository) replay(i Idempotency) (string, bool, error) {
	record, ok := m.idempotency[i.Operation+"\x00"+i.Key]
	if !ok {
		return "", false, nil
	}
	if record.requestHash != i.RequestHash {
		return "", false, ErrIdempotencyConflict
	}
	return record.resourceID, true, nil
}

func (m *MemoryRepository) remember(i Idempotency, resourceID string) {
	m.idempotency[i.Operation+"\x00"+i.Key] = idempotencyRecord{i.RequestHash, resourceID}
}

func (m *MemoryRepository) CreateProject(_ context.Context, value Project, idem Idempotency) (Project, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, found, err := m.replay(idem); found || err != nil {
		return m.projects[id], found, err
	}
	m.projects[value.ID] = value
	m.remember(idem, value.ID)
	return value, false, nil
}

func (m *MemoryRepository) GetProject(_ context.Context, id string) (Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.projects[id]
	if !ok {
		return Project{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) CreateTarget(_ context.Context, value Target, idem Idempotency) (Target, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, found, err := m.replay(idem); found || err != nil {
		return m.targets[id], found, err
	}
	m.targets[value.ID] = value
	m.remember(idem, value.ID)
	return value, false, nil
}

func (m *MemoryRepository) GetTarget(_ context.Context, projectID, id string) (Target, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.targets[id]
	if !ok || value.ProjectID != projectID {
		return Target{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) CreateQueuedRun(_ context.Context, run Run, job JobRecord, outbox OutboxMessage, idem Idempotency) (Run, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, found, err := m.replay(idem); found || err != nil {
		return m.runs[id], found, err
	}
	m.runs[run.ID] = run
	m.jobs[job.Job.JobID] = job
	m.outbox[outbox.ID] = outbox
	m.remember(idem, run.ID)
	return run, false, nil
}

func (m *MemoryRepository) GetJob(_ context.Context, id string) (JobRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.jobs[id]
	if !ok {
		return JobRecord{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) CompleteJob(_ context.Context, jobID, completionHash string, findings []Finding, evidence []Evidence, report Report, completedAt time.Time) (Report, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	job, ok := m.jobs[jobID]
	if !ok {
		return Report{}, false, ErrNotFound
	}
	run, ok := m.runs[job.RunID]
	if !ok {
		return Report{}, false, ErrNotFound
	}
	if run.Status == "cancelled" {
		return Report{}, false, ErrCancelledRun
	}
	if job.Status == "completed" {
		if job.CompletionHash == completionHash {
			return m.reports[job.RunID], true, nil
		}
		return Report{}, false, ErrStaleCompletion
	}
	if job.Status != "queued" && job.Status != "dispatched" {
		return Report{}, false, ErrStaleCompletion
	}
	job.Status = "completed"
	job.CompletionHash = completionHash
	job.CompletedAt = &completedAt
	m.jobs[jobID] = job
	m.findings[job.RunID] = append([]Finding(nil), findings...)
	m.evidence[job.RunID] = append([]Evidence(nil), evidence...)
	m.reports[job.RunID] = report
	run.Status = "completed"
	run.ReportID = report.ReportID
	run.CompletedAt = &completedAt
	m.runs[run.ID] = run
	return report, false, nil
}

func (m *MemoryRepository) GetRun(_ context.Context, id string) (Run, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.runs[id]
	if !ok {
		return Run{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) GetReportByRun(_ context.Context, runID string) (Report, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.reports[runID]
	if !ok {
		return Report{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) SetBaseline(_ context.Context, value Baseline, idem Idempotency) (Baseline, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if projectID, found, err := m.replay(idem); found || err != nil {
		return m.baselines[projectID], found, err
	}
	m.baselines[value.ProjectID] = value
	m.remember(idem, value.ProjectID)
	return value, false, nil
}

func (m *MemoryRepository) GetBaseline(_ context.Context, projectID string) (Baseline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.baselines[projectID]
	if !ok {
		return Baseline{}, ErrNotFound
	}
	return value, nil
}

func (m *MemoryRepository) ClaimOutbox(_ context.Context, limit int, lease time.Duration) ([]OutboxMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	values := make([]OutboxMessage, 0, limit)
	for id, message := range m.outbox {
		if len(values) >= limit {
			break
		}
		if message.AvailableAt.After(now) || m.outboxLease[id].After(now) {
			continue
		}
		message.Attempts++
		m.outbox[id] = message
		m.outboxLease[id] = now.Add(lease)
		values = append(values, message)
	}
	return values, nil
}

func (m *MemoryRepository) MarkOutboxPublished(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.outbox[id]; !ok {
		return ErrNotFound
	}
	delete(m.outbox, id)
	delete(m.outboxLease, id)
	return nil
}

func (m *MemoryRepository) MarkOutboxFailed(_ context.Context, id string, availableAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	message, ok := m.outbox[id]
	if !ok {
		return ErrNotFound
	}
	message.AvailableAt = availableAt
	m.outbox[id] = message
	delete(m.outboxLease, id)
	return nil
}

func (m *MemoryRepository) Ping(ctx context.Context) error {
	return ctx.Err()
}
