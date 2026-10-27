-- 000001_init.up.sql: Phase 1 ka base schema - workflows, workflow_runs, tasks.
-- `migrate -path migrations -database "..." up` se apply hota hai.
-- Undo karna ho to iska pair 000001_init.down.sql hai (reverse order me DROP).

CREATE TABLE workflows (
  id          UUID PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  spec        JSONB NOT NULL,
  enabled     BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE workflow_runs (
  id            UUID PRIMARY KEY,
  workflow_id   UUID NOT NULL REFERENCES workflows(id),
  status        TEXT NOT NULL,
  scheduled_for TIMESTAMPTZ NOT NULL,
  started_at    TIMESTAMPTZ,
  finished_at   TIMESTAMPTZ
);

CREATE TABLE tasks (
  id               UUID PRIMARY KEY,
  run_id           UUID NOT NULL REFERENCES workflow_runs(id),
  name             TEXT NOT NULL,
  type             TEXT NOT NULL,
  payload          JSONB NOT NULL,
  status           TEXT NOT NULL,
  attempt          INT  NOT NULL DEFAULT 0,
  max_attempts     INT  NOT NULL DEFAULT 3,
  run_after        TIMESTAMPTZ NOT NULL DEFAULT now(),
  idempotency_key  TEXT NOT NULL,
  last_error       TEXT,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX tasks_ready_idx ON tasks (run_after) WHERE status = 'READY';
