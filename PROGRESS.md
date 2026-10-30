# Forgeflow — Progress Tracker

This file tracks what has been completed on the `main` branch so far,
so that every new feature starts on its own branch with a clear, known
starting point.

## `main` branch — completed so far (merged)

- [x] Go module + project structure (`cmd/`, `internal/`, `proto/`, `migrations/`)
- [x] `scheduler.proto` — full contract defined (SubmitWorkflow, PollTask, ReportResult)
- [x] Postgres schema + migrations applied (`workflows`, `workflow_runs`, `tasks`)
- [x] `docker-compose.yml` — Postgres container
- [x] `internal/db` — pgxpool connection pool
- [x] `SubmitWorkflow` RPC — fully implemented (transaction + dependency resolution)
- [x] `TestSubmitWorkflow` — integration test, passing
- [x] Connected to GitHub repo and pushed

- [x] `PollTask` RPC — worker pulls a task from the queue (`FOR UPDATE SKIP LOCKED`)
- [x] `TestPollTask` — integration test, passing (verifies claim + no double-claim)
- [x] `feature/poll-task` merged into `main`

- [x] `task_deps` table (migration `000002_task_deps.up.sql`) + `SubmitWorkflow` populates it
- [x] `ReportResult` RPC — fencing-token check, task DONE + dependent wake-up (success),
      retry (`READY`) or permanent `FAILED` based on `max_attempts` (failure)
- [x] `TestReportResult`, `TestReportResultFailureRetry` — integration tests, passing

- [x] Real gRPC server listener (`cmd/scheduler/main.go`) — `net.Listen(":50051")` + `grpc.NewServer()`
- [x] Worker service (`cmd/worker/main.go`) — gRPC client, infinite poll loop with
      backoff on empty/error, simulated task execution (`time.Sleep`), `ReportResult`
      reporting. Verified end-to-end live: worker claims task, reports success,
      scheduler marks it DONE and wakes up the dependent task (DAG resolution
      confirmed outside of tests too).

## Up next (not started yet, pick one at a time)

- [ ] Real `sleep`/`http` task executors in the worker (abhi sirf simulated `time.Sleep`)
- [ ] Lease / heartbeat / fencing token (Phase 2)

### Low priority (not needed right now)

- [ ] Exponential backoff on retry (`run_after` should move into the future, not now())

## Branch naming convention

Every new feature gets its own branch:
```
feature/poll-task
feature/report-result
feature/worker-service
```

## Workflow to follow

1. `main` should always contain **working, tested** code only
2. Before starting new work: create a new branch off `main`
3. Do the work and write/pass tests on that branch
4. Merge back into `main` only once tests pass
5. Update this file (`PROGRESS.md`) — check off what was completed
