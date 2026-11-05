#!/usr/bin/env bash
# Jobflow — all setup commands run so far, in one place.
# Run these yourself, top to bottom, step by step (copy-paste them, or via ./SETUP_COMMANDS.sh).
# Run this from inside this folder (jobflow/).

set -e  # stop the script immediately if any command fails

# ---------------------------------------------------------------------------
# STEP 1: Go module init
# What it does: turns the project into a "Go module", creates a go.mod file
# so Go knows this is a proper project and packages can be imported.
# (This only runs once — if go.mod already exists, don't run it again)
# ---------------------------------------------------------------------------
# go mod init jobflow


# ---------------------------------------------------------------------------
# STEP 2: Install protoc (protobuf compiler) — system tool, via dnf on Rocky Linux
# What it does: installs the main tool that reads/compiles .proto files.
# ---------------------------------------------------------------------------
sudo dnf install -y protobuf-compiler


# ---------------------------------------------------------------------------
# STEP 3: Install protoc's Go plugins
# What it does: protoc can't generate Go code without these two plugins.
#   protoc-gen-go       -> turns .proto "message" blocks into Go structs
#   protoc-gen-go-grpc  -> turns .proto "service" blocks into Go gRPC interfaces
# ---------------------------------------------------------------------------
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


# ---------------------------------------------------------------------------
# STEP 4: Generate actual Go code from the .proto file
# What it does: reads proto/scheduler/v1/scheduler.proto and generates 2 files:
#   scheduler.pb.go       -> messages (structs)
#   scheduler_grpc.pb.go  -> service (gRPC interfaces)
# Run this again whenever the .proto file changes.
# ---------------------------------------------------------------------------
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/scheduler/v1/scheduler.proto


# ---------------------------------------------------------------------------
# STEP 5: Run the local Postgres container (via docker-compose)
# What it does: reads docker-compose.yml and starts a Postgres 16 container,
# available at localhost:5432 (user/pass/db = jobflow/jobflow/jobflow).
# -d = detached mode, keeps running in the background.
# ---------------------------------------------------------------------------
docker compose up -d

# To check status (whether the container is running or not):
docker compose ps


# ---------------------------------------------------------------------------
# STEP 6 (CHECK): Check whether the database has any tables yet
# What it does: runs psql (Postgres's client tool) inside the container,
# and lists all tables with \dt (a psql meta-command, not SQL).
# Output on an empty DB: "Did not find any relations."
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 7: Install the golang-migrate CLI
# What it does: installs the tool that applies the .sql files in migrations/
# to Postgres, in order.
# -tags 'postgres' => build with only the Postgres driver (smaller/faster binary)
# ---------------------------------------------------------------------------
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest


# ---------------------------------------------------------------------------
# STEP 8: Apply the migration (create the actual tables)
# What it does: runs migrations/000001_init.up.sql against Postgres,
# creating the workflows, workflow_runs, tasks tables + indexes.
#
# Connection string format: postgres://<user>:<password>@<host>:<port>/<dbname>?sslmode=disable
#   jobflow:jobflow -> user/password set in docker-compose.yml
#   localhost:5432  -> we mapped port 5432:5432, so it's reachable via localhost
#   sslmode=disable -> SSL isn't needed for local dev
#
# "up" = apply pending migrations (opposite: "down" = undo/rollback)
# ---------------------------------------------------------------------------
migrate -path migrations \
  -database "postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" \
  up


# ---------------------------------------------------------------------------
# STEP 9 (CHECK): Verify again whether the tables now exist
# Expected output: workflows, workflow_runs, tasks table names should appear.
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 10: Install pgx (Go <-> Postgres connection library)
# What it does: the Scheduler service will talk to Postgres using this
# library (connection pool, raw SQL queries, SKIP LOCKED queue queries, etc.)
# ---------------------------------------------------------------------------
go get github.com/jackc/pgx/v5/pgxpool


# ---------------------------------------------------------------------------
# STEP 11: Install gRPC/protobuf runtime libraries
# What it does: the generated files (scheduler.pb.go, scheduler_grpc.pb.go)
# use these libraries at runtime themselves (different from the tools above -
# those only generate code, these are the actual dependencies the project needs).
# ---------------------------------------------------------------------------
go get google.golang.org/grpc
go get google.golang.org/protobuf


# ---------------------------------------------------------------------------
# STEP 12 (CHECK): Run the scheduler's first real Go code to test it
# What it does: sets the DATABASE_URL env variable and runs cmd/scheduler/main.go
# - it creates a pool to Postgres and prints a confirmation.
# Expected output: "connected to database successfully"
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go run ./cmd/scheduler


# ---------------------------------------------------------------------------
# STEP 13: Install the uuid library
# What it does: used in SubmitWorkflow to randomly generate new
# workflow/run/task IDs (uuid.New().String()).
# ---------------------------------------------------------------------------
go get github.com/google/uuid


# ---------------------------------------------------------------------------
# STEP 14 (CHECK): Run the SubmitWorkflow test
# What it does: runs the TestSubmitWorkflow function in
# internal/scheduler/server_test.go - connects to real Postgres and calls
# SubmitWorkflow() directly (without running the gRPC server) and checks the result.
# -run <name>  => run only this test
# -v           => verbose output (t.Logf etc. will also show)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestSubmitWorkflow -v


# ---------------------------------------------------------------------------
# STEP 15 (CHECK): Manually verify the data with psql
# What it does: opens a psql shell inside the Postgres container, so we can
# see for ourselves whether SubmitWorkflow inserted the right data into the
# workflows/workflow_runs/tasks tables.
# ---------------------------------------------------------------------------
docker exec -it $(docker ps --filter "name=postgres" -q) psql -U jobflow -d jobflow

# Queries to run inside the psql shell (copy the workflow_id from the test
# output above, from the "workflow created with id: ..." line):
#
# SELECT id, name FROM workflows WHERE id = '<workflow_id>';
# SELECT * FROM workflow_runs WHERE workflow_id = '<workflow_id>';
# SELECT name, status FROM tasks WHERE run_id = (SELECT id FROM workflow_runs WHERE workflow_id = '<workflow_id>');
#
# Expected: task-A -> READY, task-B -> PENDING (since task-B depends on task-A)


# ---------------------------------------------------------------------------
# STEP 16 (CHECK): Run the PollTask test
# What it does: runs the TestPollTask function in
# internal/scheduler/server_test.go - first creates a task via SubmitWorkflow
# (in READY status), then calls PollTask() twice to verify:
#   1st call  => should get the READY task (HasTask = true)
#   2nd call  => the same task should NOT be returned again (HasTask = false),
#                because after the first call its status became RUNNING
#                (FOR UPDATE SKIP LOCKED + UPDATE concurrency-safety check)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestPollTask -v


# ---------------------------------------------------------------------------
# STEP 17: Apply pending migrations (whenever a new migrations/*.up.sql
# file is created, e.g. 000002_task_deps.up.sql)
# What it does: checks the schema_migrations table to see which migrations
# are already applied, and runs only the remaining pending files.
# ---------------------------------------------------------------------------
migrate -path migrations -database "postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" up


# ---------------------------------------------------------------------------
# STEP 18 (CHECK): List all tables (one-line command, without entering the
# psql shell)
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 19 (CHECK): View the data in the tasks and task_deps tables
# What it does:
#  - first query: shows each task's name, id, status
#  - second query: JOINs task_deps with tasks to show human-readable
#    "which task depends on which" (no need to look at raw UUIDs)
# Expected (after TestSubmitWorkflow runs): task-A -> READY, task-B -> PENDING,
# and task-B depends_on task-A
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "SELECT name, id, status FROM tasks;"

docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "SELECT t1.name AS task, t2.name AS depends_on FROM task_deps d JOIN tasks t1 ON d.task_id = t1.id JOIN tasks t2 ON d.depends_on_id = t2.id;"


# ---------------------------------------------------------------------------
# STEP 20 (CHECK): Run the ReportResult test
# What it does: runs the TestReportResult and TestReportResultFailureRetry
# functions in internal/scheduler/server_test.go.
#   TestReportResult            => success flow: stale fencing rejected,
#                                  task DONE, dependent task READY (wake-up)
#   TestReportResultFailureRetry => failure flow: attempt < max_attempts goes
#                                  READY (retry), attempt >= max_attempts
#                                  goes FAILED (permanent)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestReportResult -v


# ---------------------------------------------------------------------------
# STEP 21: Load DATABASE_URL from the .env file (to avoid typing
# DATABASE_URL="..." every time)
# What it does: the .env file has one line (DATABASE_URL=postgres://...),
# this command reads it and sets the env variable in the current shell session.
# After this you can run commands directly without the DATABASE_URL="..." prefix,
# e.g.: go test ./internal/scheduler/... -run TestWorkerExecutorHTTP -v
# Note: this only works for this terminal session - if you open a new
# terminal you'll need to run it again.
# ---------------------------------------------------------------------------
export $(cat .env | xargs)


# ---------------------------------------------------------------------------
# STEP 22 (CHECK): Run the http executor test
# What it does: runs the TestWorkerExecutorHTTP function in
# internal/scheduler/server_test.go - starts a local fake HTTP server,
# submits an "http" type task, picks it up via PollTask,
# actually sends the HTTP request via executors.RunHTTP(), and verifies the
# task status is DONE after ReportResult.
# ---------------------------------------------------------------------------
go test ./internal/scheduler/... -run TestWorkerExecutorHTTP -v -count=1


# ---------------------------------------------------------------------------
# STEP 23: Apply the new migration (000003_task_lease — adds the
# lease_owner, lease_expires_at columns to the tasks table, for Phase 2)
# ---------------------------------------------------------------------------
migrate -path migrations -database "postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" up


# ---------------------------------------------------------------------------
# STEP 24 (CHECK): Check the tasks table structure to verify that the
# lease_owner and lease_expires_at columns were added
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\d tasks"
