#!/usr/bin/env bash
# Jobflow — ab tak chalai gayi saari setup commands, ek jagah.
# Ise top se neeche, step by step, khud chalao (copy-paste karke ya ./SETUP_COMMANDS.sh se).
# Run isi folder (jobflow/) ke andar se karna.

set -e  # koi bhi command fail ho to script turant ruk jaye

# ---------------------------------------------------------------------------
# STEP 1: Go module init
# Kya karta hai: project ko ek "Go module" banata hai, go.mod file banti hai
# jisse Go ko pata chale ye ek proper project hai aur packages import ho sakein.
# (Ye sirf ek baar chalti hai — agar go.mod pehle se hai to dobara mat chalana)
# ---------------------------------------------------------------------------
# go mod init jobflow


# ---------------------------------------------------------------------------
# STEP 2: protoc (protobuf compiler) install karna — system tool, Rocky Linux pe dnf se
# Kya karta hai: .proto files ko padhne/compile karne wala main tool install karta hai.
# ---------------------------------------------------------------------------
sudo dnf install -y protobuf-compiler


# ---------------------------------------------------------------------------
# STEP 3: protoc ke Go plugins install karna
# Kya karta hai: in dono ke bina protoc ko pata nahi chalega Go code kaise banayein.
#   protoc-gen-go       -> .proto ke "message" blocks ko Go structs banata hai
#   protoc-gen-go-grpc  -> .proto ke "service" block ko Go gRPC interfaces banata hai
# ---------------------------------------------------------------------------
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


# ---------------------------------------------------------------------------
# STEP 4: .proto file se asli Go code generate karna
# Kya karta hai: proto/scheduler/v1/scheduler.proto ko padh ke 2 files banata hai:
#   scheduler.pb.go       -> messages (structs)
#   scheduler_grpc.pb.go  -> service (gRPC interfaces)
# Jab bhi .proto file badlo, ye command dobara chalani padti hai.
# ---------------------------------------------------------------------------
protoc \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/scheduler/v1/scheduler.proto


# ---------------------------------------------------------------------------
# STEP 5: Local Postgres container chalana (docker-compose se)
# Kya karta hai: docker-compose.yml padh ke Postgres 16 ka container start karta hai,
# localhost:5432 pe available hoga (user/pass/db = jobflow/jobflow/jobflow).
# -d = detached mode, background me chalta rahega.
# ---------------------------------------------------------------------------
docker compose up -d

# Status check karne ke liye (container chal raha hai ya nahi):
docker compose ps


# ---------------------------------------------------------------------------
# STEP 6 (CHECK): Database me abhi koi table hai ya nahi, dekhna
# Kya karta hai: container ke andar psql (Postgres ka client tool) chalata hai,
# aur \dt (ek psql meta-command, SQL nahi) se saari tables list karta hai.
# Khaali DB pe output: "Did not find any relations."
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 7: golang-migrate CLI install karna
# Kya karta hai: migrations/ folder ki .sql files ko order se Postgres pe apply
# karne wala tool install karta hai.
# -tags 'postgres' => sirf Postgres driver ke saath build karo (chhota/fast binary)
# ---------------------------------------------------------------------------
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest


# ---------------------------------------------------------------------------
# STEP 8: Migration apply karna (asli tables banana)
# Kya karta hai: migrations/000001_init.up.sql ko Postgres pe chalata hai,
# jisse workflows, workflow_runs, tasks tables + index ban jate hain.
#
# Connection string format: postgres://<user>:<password>@<host>:<port>/<dbname>?sslmode=disable
#   jobflow:jobflow -> docker-compose.yml me set kiya hua user/password
#   localhost:5432  -> humne port 5432:5432 map kiya tha, isliye localhost se dikhega
#   sslmode=disable -> local dev me SSL ki zaroorat nahi
#
# "up" = pending migrations apply karo (opposite: "down" = undo/rollback karo)
# ---------------------------------------------------------------------------
migrate -path migrations \
  -database "postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" \
  up


# ---------------------------------------------------------------------------
# STEP 9 (CHECK): Ab tables bani hain ya nahi, dobara verify karna
# Expected output: workflows, workflow_runs, tasks table names dikhni chahiye.
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 10: pgx install karna (Go <-> Postgres connection library)
# Kya karta hai: Scheduler service isi library se Postgres se baat karega
# (connection pool, raw SQL queries, SKIP LOCKED wali queue queries, etc.)
# ---------------------------------------------------------------------------
go get github.com/jackc/pgx/v5/pgxpool


# ---------------------------------------------------------------------------
# STEP 11: gRPC/protobuf runtime libraries install karna
# Kya karta hai: generated files (scheduler.pb.go, scheduler_grpc.pb.go) khud
# in libraries ko use karte hain runtime pe (tools se alag - wo sirf code
# generate karte hain, ye asli dependency hain jo project ko chahiye).
# ---------------------------------------------------------------------------
go get google.golang.org/grpc
go get google.golang.org/protobuf


# ---------------------------------------------------------------------------
# STEP 12 (CHECK): Scheduler ka pehla asli Go code run karke test karna
# Kya karta hai: DATABASE_URL env variable set karke cmd/scheduler/main.go
# chalata hai - ye Postgres se pool banata hai aur confirm print karta hai.
# Expected output: "connected to database successfully"
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go run ./cmd/scheduler


# ---------------------------------------------------------------------------
# STEP 13: uuid library install karna
# Kya karta hai: SubmitWorkflow me naye workflow/run/task IDs random generate
# karne ke liye use hoti hai (uuid.New().String()).
# ---------------------------------------------------------------------------
go get github.com/google/uuid


# ---------------------------------------------------------------------------
# STEP 14 (CHECK): SubmitWorkflow ka test chalana
# Kya karta hai: internal/scheduler/server_test.go me likha TestSubmitWorkflow
# function run karta hai - real Postgres se connect hoke SubmitWorkflow()
# ko directly call karta hai (bina gRPC server chalaye) aur result check karta hai.
# -run <naam>  => sirf isi test ko chalao
# -v           => verbose output (t.Logf wagera bhi dikhega)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestSubmitWorkflow -v


# ---------------------------------------------------------------------------
# STEP 15 (CHECK): psql se manually data verify karna
# Kya karta hai: Postgres container ke andar jaake psql shell kholta hai,
# taaki hum khud dekh sakein ki SubmitWorkflow ne workflows/workflow_runs/
# tasks tables me sahi data daala ya nahi.
# ---------------------------------------------------------------------------
docker exec -it $(docker ps --filter "name=postgres" -q) psql -U jobflow -d jobflow

# psql shell ke andar chalane wali queries (workflow_id upar wale test ke
# output se copy karo, "workflow created with id: ..." wali line se):
#
# SELECT id, name FROM workflows WHERE id = '<workflow_id>';
# SELECT * FROM workflow_runs WHERE workflow_id = '<workflow_id>';
# SELECT name, status FROM tasks WHERE run_id = (SELECT id FROM workflow_runs WHERE workflow_id = '<workflow_id>');
#
# Expected: task-A -> READY, task-B -> PENDING (kyunki task-B, task-A pe depend karta hai)


# ---------------------------------------------------------------------------
# STEP 16 (CHECK): PollTask ka test chalana
# Kya karta hai: internal/scheduler/server_test.go me likha TestPollTask
# function run karta hai - pehle SubmitWorkflow se ek task banata hai (READY
# status me), phir PollTask() ko 2 baar call karke verify karta hai:
#   1st call  => READY task mil jaana chahiye (HasTask = true)
#   2nd call  => wahi task dobara nahi milna chahiye (HasTask = false),
#                kyunki pehli call ke baad uska status RUNNING ho chuka hoga
#                (FOR UPDATE SKIP LOCKED + UPDATE ka concurrency-safety check)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestPollTask -v


# ---------------------------------------------------------------------------
# STEP 17: Pending migrations apply karna (jab bhi naya migrations/*.up.sql
# file banaya ho, jaise 000002_task_deps.up.sql)
# Kya karta hai: schema_migrations table check karta hai ki konsi migrations
# already apply ho chuki hain, aur sirf baaki pending wali files chalata hai.
# ---------------------------------------------------------------------------
migrate -path migrations -database "postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" up


# ---------------------------------------------------------------------------
# STEP 18 (CHECK): Saari tables ki list dekhna (ek-line command, psql shell
# ke andar jaaye bina)
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "\dt"


# ---------------------------------------------------------------------------
# STEP 19 (CHECK): tasks aur task_deps table ka data dekhna
# Kya karta hai:
#  - pehli query: har task ka naam, id, status dikhati hai
#  - doosri query: task_deps ko tasks ke saath JOIN karke human-readable
#    dikhati hai "kaunsa task kiske upar depend karta hai" (UUID dekhne ki
#    zaroorat nahi padti)
# Expected (after TestSubmitWorkflow run): task-A -> READY, task-B -> PENDING,
# aur task-B depends_on task-A
# ---------------------------------------------------------------------------
docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "SELECT name, id, status FROM tasks;"

docker exec -it jobflow-postgres-1 psql -U jobflow -d jobflow -c "SELECT t1.name AS task, t2.name AS depends_on FROM task_deps d JOIN tasks t1 ON d.task_id = t1.id JOIN tasks t2 ON d.depends_on_id = t2.id;"


# ---------------------------------------------------------------------------
# STEP 20 (CHECK): ReportResult ka test chalana
# Kya karta hai: internal/scheduler/server_test.go me likhe TestReportResult
# aur TestReportResultFailureRetry functions run karta hai.
#   TestReportResult            => success flow: stale fencing reject,
#                                  task DONE, dependent task READY (wake-up)
#   TestReportResultFailureRetry => failure flow: attempt < max_attempts par
#                                  READY (retry), attempt >= max_attempts
#                                  par FAILED (permanent)
# ---------------------------------------------------------------------------
DATABASE_URL="postgres://jobflow:jobflow@localhost:5432/jobflow?sslmode=disable" go test ./internal/scheduler/... -run TestReportResult -v
