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

## Up next (not started yet, pick one at a time)

- [ ] `ReportResult` RPC — worker reports the task result back
- [ ] Start the actual gRPC server (real listener in `main.go`)
- [ ] Worker service (`cmd/worker/main.go`) — poll loop + `sleep`/`http` executors
- [ ] Lease / heartbeat / fencing token (Phase 2)

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
