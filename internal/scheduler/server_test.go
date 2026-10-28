package scheduler

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"jobflow/internal/db"
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