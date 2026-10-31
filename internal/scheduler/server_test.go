package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"jobflow/internal/db"
	"jobflow/internal/worker/executors"
	schedulerv1 "jobflow/proto/scheduler/v1"
)

// cleanDB: purane test runs ka leftover data hata deta hai, taaki har test
// fresh/empty state se shuru ho (warna purane READY/PENDING tasks naye
// assertions ko (jaise "ab koi READY task nahi bachega") galat prove kar dete hain).
func cleanDB(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `TRUNCATE tasks, workflow_runs, workflows CASCADE`)
	if err != nil {
		t.Fatalf("failed to clean db: %v", err)
	}
}

func TestSubmitWorkflow(t *testing.T) {
	
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	cleanDB(t, ctx, pool)

	server := NewServer(pool)

	req := &schedulerv1.SubmitWorkflowRequest{
		Name: "test-workflow-" + uuid.New().String(),
		Tasks: []*schedulerv1.TaskSpec{
			{Name: "task-A", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`)},
			{Name: "task-B", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`), DependsOn: []string{"task-A"}},
		},
	}

	resp, err := server.SubmitWorkflow(ctx, req)
	if err != nil {
		t.Fatalf("SubmitWorkflow failed: %v", err)
	}

	if resp.WorkflowId == "" {
		t.Fatal("expected non-empty workflow_id")
	}

	t.Logf("workflow created with id: %s", resp.WorkflowId)
}


func TestPollTask(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	cleanDB(t, ctx, pool)

	server := NewServer(pool)

	submitResp, err := server.SubmitWorkflow(ctx, &schedulerv1.SubmitWorkflowRequest{
		Name: "poll-test-workflow-" + uuid.New().String(),
		
		Tasks: []*schedulerv1.TaskSpec{
			{Name: "task-A", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`)},
			{Name: "task-B", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`), DependsOn: []string{"task-A"}},
		},
	})
	if err != nil {
		t.Fatalf("SubmitWorkflow failed: %v", err)
	}
	_ = submitResp


	pollResp, err := server.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "test-worker"})

	if err != nil{
		t.Fatalf("PollTask failed: %v", err)
	}

	if !pollResp.HasTask{
		t.Fatal("expected a task to be claimed, got HasTask=false")
	}

	pollResp2, err := server.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "test-worker"})
	if err != nil {
		t.Fatalf("second PollTask failed: %v", err)
	}
	if pollResp2.HasTask {
		t.Fatal("expected no task available, got HasTask=true")
	}
}

func TestReportResult(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	cleanDB(t, ctx, pool)

	server := NewServer(pool)

	_, err = server.SubmitWorkflow(ctx, &schedulerv1.SubmitWorkflowRequest{
		Name: "report-result-workflow-" + uuid.New().String(),
		Tasks: []*schedulerv1.TaskSpec{
			{Name: "task-A", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`)},
			{Name: "task-B", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`), DependsOn: []string{"task-A"}},
		},
	})
	if err != nil {
		t.Fatalf("SubmitWorkflow failed: %v", err)
	}

	// task-A hi READY hai (task-B abhi PENDING hai), isliye PollTask se task-A milega
	pollResp, err := server.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "test-worker"})
	if err != nil {
		t.Fatalf("PollTask failed: %v", err)
	}
	if !pollResp.HasTask {
		t.Fatal("expected a task to be claimed, got HasTask=false")
	}

	taskAID := pollResp.Task.Id
	taskAAttempt := pollResp.Task.Attempt

	// stale/zombie result: galat attempt number se report karo, reject hona chahiye
	staleResp, err := server.ReportResult(ctx, &schedulerv1.ReportResultRequest{
		TaskId:  taskAID,
		Attempt: taskAAttempt + 1, // DB me jo attempt hai usse alag
		Success: true,
	})
	if err != nil {
		t.Fatalf("ReportResult (stale) failed: %v", err)
	}
	if staleResp.Accepted {
		t.Fatal("expected stale ReportResult to be rejected, got Accepted=true")
	}

	// ab sahi attempt se success report karo
	okResp, err := server.ReportResult(ctx, &schedulerv1.ReportResultRequest{
		TaskId:  taskAID,
		Attempt: taskAAttempt,
		Success: true,
	})
	if err != nil {
		t.Fatalf("ReportResult failed: %v", err)
	}
	if !okResp.Accepted {
		t.Fatal("expected ReportResult to be accepted, got Accepted=false")
	}

	// task-A ab DONE hona chahiye
	var statusA string
	err = pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, taskAID).Scan(&statusA)
	if err != nil {
		t.Fatalf("failed to fetch task-A status: %v", err)
	}
	if statusA != "DONE" {
		t.Fatalf("expected task-A status DONE, got %s", statusA)
	}

	// task-B ab READY hona chahiye (uski ek hi dependency thi, wo DONE ho chuki)
	var statusB string
	err = pool.QueryRow(ctx, `SELECT status FROM tasks WHERE name = 'task-B'`).Scan(&statusB)
	if err != nil {
		t.Fatalf("failed to fetch task-B status: %v", err)
	}
	if statusB != "READY" {
		t.Fatalf("expected task-B status READY, got %s", statusB)
	}
}

func TestReportResultFailureRetry(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	cleanDB(t, ctx, pool)

	server := NewServer(pool)

	_, err = server.SubmitWorkflow(ctx, &schedulerv1.SubmitWorkflowRequest{
		Name: "report-result-retry-workflow-" + uuid.New().String(),
		Tasks: []*schedulerv1.TaskSpec{
			{Name: "task-A", Type: "sleep", Payload: []byte(`{"duration_seconds": 1}`)},
		},
	})
	if err != nil {
		t.Fatalf("SubmitWorkflow failed: %v", err)
	}

	pollResp, err := server.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "test-worker"})
	if err != nil {
		t.Fatalf("PollTask failed: %v", err)
	}
	if !pollResp.HasTask {
		t.Fatal("expected a task to be claimed, got HasTask=false")
	}

	taskID := pollResp.Task.Id
	attempt := pollResp.Task.Attempt

	// default max_attempts = 3, aur abhi attempt 0 hai, isliye retry (READY) hona chahiye
	failResp, err := server.ReportResult(ctx, &schedulerv1.ReportResultRequest{
		TaskId:  taskID,
		Attempt: attempt,
		Success: false,
		Error:   "boom",
	})
	if err != nil {
		t.Fatalf("ReportResult (failure) failed: %v", err)
	}
	if !failResp.Accepted {
		t.Fatal("expected failure ReportResult to be accepted, got Accepted=false")
	}

	var status, lastError string
	err = pool.QueryRow(ctx, `SELECT status, last_error FROM tasks WHERE id = $1`, taskID).Scan(&status, &lastError)
	if err != nil {
		t.Fatalf("failed to fetch task status: %v", err)
	}
	if status != "READY" {
		t.Fatalf("expected task status READY (retry), got %s", status)
	}
	if lastError != "boom" {
		t.Fatalf("expected last_error 'boom', got %s", lastError)
	}

	// max_attempts tak attempt force karo, taaki agli failure permanently FAILED ho
	_, err = pool.Exec(ctx, `UPDATE tasks SET attempt = max_attempts WHERE id = $1`, taskID)
	if err != nil {
		t.Fatalf("failed to force attempt to max: %v", err)
	}

	var forcedAttempt int32
	err = pool.QueryRow(ctx, `SELECT attempt FROM tasks WHERE id = $1`, taskID).Scan(&forcedAttempt)
	if err != nil {
		t.Fatalf("failed to fetch forced attempt: %v", err)
	}

	finalFailResp, err := server.ReportResult(ctx, &schedulerv1.ReportResultRequest{
		TaskId:  taskID,
		Attempt: forcedAttempt,
		Success: false,
		Error:   "boom again",
	})
	if err != nil {
		t.Fatalf("ReportResult (final failure) failed: %v", err)
	}
	if !finalFailResp.Accepted {
		t.Fatal("expected final failure ReportResult to be accepted, got Accepted=false")
	}

	var finalStatus string
	err = pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, taskID).Scan(&finalStatus)
	if err != nil {
		t.Fatalf("failed to fetch final task status: %v", err)
	}
	if finalStatus != "FAILED" {
		t.Fatalf("expected task status FAILED, got %s", finalStatus)
	}
}

// TestWorkerExecutorHTTP: end-to-end check ki "http" type task scheduler se
// mil ke executors.RunHTTP se actually chal sake, aur ReportResult ke baad
// status DONE ho jaaye.
func TestWorkerExecutorHTTP(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	cleanDB(t, ctx, pool)

	// Ek local fake HTTP server, taaki test internet pe depend na kare.
	fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer fakeServer.Close()

	server := NewServer(pool)

	payload := []byte(`{"method":"GET","url":"` + fakeServer.URL + `"}`)

	_, err = server.SubmitWorkflow(ctx, &schedulerv1.SubmitWorkflowRequest{
		Name: "http-executor-workflow-" + uuid.New().String(),
		Tasks: []*schedulerv1.TaskSpec{
			{Name: "task-A", Type: "http", Payload: payload},
		},
	})
	if err != nil {
		t.Fatalf("SubmitWorkflow failed: %v", err)
	}

	pollResp, err := server.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "test-worker"})
	if err != nil {
		t.Fatalf("PollTask failed: %v", err)
	}
	if !pollResp.HasTask {
		t.Fatal("expected a task to be claimed, got HasTask=false")
	}
	if pollResp.Task.Type != "http" {
		t.Fatalf("expected task type 'http', got %s", pollResp.Task.Type)
	}

	// Yahi wo cheez hai jo worker/main.go bhi karta hai: task mila, ab
	// uske type ke hisaab se sahi executor call karo.
	taskErr := executors.RunHTTP(pollResp.Task.Payload, pollResp.Task.IdempotencyKey)
	if taskErr != nil {
		t.Fatalf("RunHTTP failed: %v", taskErr)
	}

	okResp, err := server.ReportResult(ctx, &schedulerv1.ReportResultRequest{
		TaskId:  pollResp.Task.Id,
		Attempt: pollResp.Task.Attempt,
		Success: true,
	})
	if err != nil {
		t.Fatalf("ReportResult failed: %v", err)
	}
	if !okResp.Accepted {
		t.Fatal("expected ReportResult to be accepted, got Accepted=false")
	}

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, pollResp.Task.Id).Scan(&status)
	if err != nil {
		t.Fatalf("failed to fetch task status: %v", err)
	}
	if status != "DONE" {
		t.Fatalf("expected task status DONE, got %s", status)
	}
}