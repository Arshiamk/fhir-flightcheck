CREATE TABLE IF NOT EXISTS schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
    organization_id text NOT NULL,
    id text NOT NULL,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, id)
);
CREATE TABLE IF NOT EXISTS targets (
    organization_id text NOT NULL,
    id text NOT NULL,
    project_id text NOT NULL,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, id),
    FOREIGN KEY (organization_id, project_id) REFERENCES projects (organization_id, id)
);
CREATE TABLE IF NOT EXISTS runs (
    organization_id text NOT NULL,
    id text NOT NULL,
    project_id text NOT NULL,
    target_id text NOT NULL,
    status text NOT NULL,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, id),
    FOREIGN KEY (organization_id, project_id) REFERENCES projects (organization_id, id),
    FOREIGN KEY (organization_id, target_id) REFERENCES targets (organization_id, id)
);
CREATE TABLE IF NOT EXISTS jobs (
    organization_id text NOT NULL,
    id text NOT NULL,
    run_id text NOT NULL,
    status text NOT NULL,
    completion_hash text,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    completed_at timestamptz,
    PRIMARY KEY (organization_id, id),
    FOREIGN KEY (organization_id, run_id) REFERENCES runs (organization_id, id)
);
CREATE TABLE IF NOT EXISTS reports (
    organization_id text NOT NULL,
    report_id text NOT NULL,
    run_id text NOT NULL,
    body jsonb NOT NULL,
    created_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, report_id),
    UNIQUE (organization_id, run_id),
    FOREIGN KEY (organization_id, run_id) REFERENCES runs (organization_id, id)
);
CREATE TABLE IF NOT EXISTS findings (
    organization_id text NOT NULL,
    finding_id text NOT NULL,
    run_id text NOT NULL,
    rule_id text NOT NULL,
    body jsonb NOT NULL,
    PRIMARY KEY (organization_id, finding_id),
    UNIQUE (organization_id, run_id, rule_id),
    FOREIGN KEY (organization_id, run_id) REFERENCES runs (organization_id, id)
);
CREATE TABLE IF NOT EXISTS evidence (
    organization_id text NOT NULL,
    evidence_id text NOT NULL,
    run_id text NOT NULL,
    body jsonb NOT NULL,
    PRIMARY KEY (organization_id, evidence_id),
    FOREIGN KEY (organization_id, run_id) REFERENCES runs (organization_id, id)
);
CREATE TABLE IF NOT EXISTS baselines (
    organization_id text NOT NULL,
    project_id text NOT NULL,
    run_id text NOT NULL,
    report_id text NOT NULL,
    body jsonb NOT NULL,
    PRIMARY KEY (organization_id, project_id),
    FOREIGN KEY (organization_id, project_id) REFERENCES projects (organization_id, id),
    FOREIGN KEY (organization_id, report_id) REFERENCES reports (organization_id, report_id)
);
CREATE TABLE IF NOT EXISTS idempotency_keys (
    organization_id text NOT NULL,
    operation text NOT NULL,
    key text NOT NULL,
    request_sha256 text NOT NULL,
    resource_id text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, operation, key)
);
CREATE TABLE IF NOT EXISTS outbox (
    organization_id text NOT NULL,
    id text NOT NULL,
    subject text NOT NULL,
    payload jsonb NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL,
    lease_until timestamptz,
    published_at timestamptz,
    last_error_at timestamptz,
    PRIMARY KEY (organization_id, id)
);
CREATE INDEX IF NOT EXISTS outbox_pending_idx
    ON outbox (available_at) WHERE published_at IS NULL;

DO $$
DECLARE table_name text;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'projects','targets','runs','jobs','reports','findings','evidence',
    'baselines','idempotency_keys','outbox'
  ] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', table_name);
    IF NOT EXISTS (
      SELECT 1 FROM pg_policies WHERE schemaname = current_schema()
        AND tablename = table_name AND policyname = 'tenant_isolation'
    ) THEN
      EXECUTE format(
        'CREATE POLICY tenant_isolation ON %I USING (organization_id = current_setting(''flightcheck.organization_id'', true)) WITH CHECK (organization_id = current_setting(''flightcheck.organization_id'', true))',
        table_name
      );
    END IF;
  END LOOP;
END $$;
