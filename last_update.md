# Last Update — Phase 2 Status (Lease / Heartbeat / Reaper)

> Ye file har session ke end me update hogi — jo us din kaam hua wahi yahan likha jayega,
> taaki agli baar seedha yahan se pata chal jaye "kahan tak pahunche the, ab kya karna hai".

---

## 2026-11-02

### Aaj kya kiya (Phase 2 ka kaam shuru kiya)

1. **Migration `000003_task_lease`** — `tasks` table me 2 naye columns add kiye:
   - `lease_owner TEXT`
   - `lease_expires_at TIMESTAMPTZ`
   - Migration `migrate ... up` se apply kar di gayi, `psql \d tasks` se verify bhi kiya — columns aa gaye.

2. **`PollTask` update** (`internal/scheduler/server.go`) — task claim karte waqt ab lease bhi di jaati hai:
   - `UPDATE tasks SET ... lease_owner = $1, lease_expires_at = now() + interval '30 seconds' ...`
   - `req.WorkerId` ab query me `$1` ke through pass hota hai.
   - Response me `Task.LeaseTtlSeconds = 30` bhi bhejna shuru kiya (worker ko batane ke liye kitni der ki lease mili).
   - `go build` aur existing `TestPollTask` dono pass ho rahe hain — purana behavior nahi toota.

3. **Proto me `Heartbeat` RPC add kiya** (`proto/scheduler/v1/scheduler.proto`):
   ```proto
   message HeartbeatRequest {
     string worker_id = 1;
     string task_id = 2;
     int32 attempt = 3;   // fencing ke liye — stale/slow worker ka heartbeat reject karne ke liye
   }

   message HeartbeatResponse {
     bool lease_valid = 1;
   }
   ```
   Aur `service SchedulerService` block me:
   ```proto
   rpc HeartBeat(HeartbeatRequest) returns (HeartbeatResponse);
   ```
   `protoc` dobara chalaya — `scheduler.pb.go` aur `scheduler_grpc.pb.go` me naya code generate ho gaya
   (`SchedulerServiceClient`/`SchedulerServiceServer` interfaces me `HeartBeat` method aa gaya).
   `go build` abhi bhi pass — kyunki `UnimplementedSchedulerServiceServer` default stub deta hai.

### `HeartBeat` RPC — implementation + tests ho gaye

- `server.go` me `HeartBeat(...)` likha gaya:
  ```sql
  UPDATE tasks
  SET lease_expires_at = now() + interval '30 seconds'
  WHERE id = $1 AND lease_owner = $2 AND attempt = $3 AND status = 'RUNNING'
  ```
  `tag.RowsAffected() == 1` → `LeaseValid: true`, `0` → `LeaseValid: false` (fencing), query error → `nil, err`.
- Tests add kiye `server_test.go` me:
  - `TestHeartBeat` — sahi worker_id + attempt se `LeaseValid=true`
  - `TestHeartBeatStale` — galat attempt aur galat worker_id dono cases me `LeaseValid=false`
- Sab 14 tests (poore project ke) pass ho rahe hain (`go test ./... -v`).

### Phase 2 — baaki kya bacha hai (roadmap se)

| Kaam | Status |
|---|---|
| `tasks` table me lease columns | ✅ Done |
| `PollTask` lease deta hai | ✅ Done |
| Proto me `Heartbeat` RPC defined | ✅ Done |
| `server.go` me `HeartBeat` ka asli logic + tests | ✅ Done |
| Worker side — background goroutine jo har ~10s `Heartbeat` bheje | ❌ Baaki |
| Reaper loop (scheduler side, expired leases ko wapas `READY` karna) | ❌ Baaki |
| Retry backoff with jitter (abhi turant `run_after = now()` hota hai) | ❌ Baaki |

### Agla step (jahan se continue karna hai)

**Worker side (`cmd/worker/main.go`) me background goroutine likhna** — jab task mile (`PollTask` se), ek goroutine start ho jo har ~10 second (lease TTL ka 1/3) `HeartBeat` RPC call kare jab tak task execute ho raha hai; task complete hone par goroutine ko gracefully stop karna hai.

### Git state

- Branch: `feature/phase2-lease-heartbeat` (is file ke commit ke saath)
- Is branch me shamil: `SETUP_COMMANDS.sh`, `internal/scheduler/server.go`, `proto/scheduler/v1/*`,
  `migrations/000003_task_lease.up.sql`, `migrations/000003_task_lease.down.sql`, ye `last_update.md`
- Abhi **`main` me merge nahi kiya**, na push kiya — explicit instruction ka wait hai.
