package scheduler

import (
	"context"
	"os"
	"testing"

	"jobflow/internal/db"
	schedulerv1 "jobflow/proto/scheduler/v1"
)

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

	server := NewServer(pool)

	req := &schedulerv1.SubmitWorkflowRequest{
		Name: "test-workflow",
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
