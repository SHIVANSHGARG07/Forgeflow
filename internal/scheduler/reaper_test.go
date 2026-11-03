package scheduler

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"jobflow/internal/db"
	schedulerv1 "jobflow/proto/scheduler/v1"
)

// TestReapExpiredLeases_Retry: task RUNNING hai, lease expire ho chuki hai,
// attempt abhi max_attempts se kam hai -> reaper use READY kar de (retry).
func TestReapExpiredLeases_Retry(t *testing.T) {
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
		Name: "reaper-retry-workflow-" + uuid.New().String(),
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

	// worker crash simulate karo: lease_expires_at ko past me daal do
	_, err = pool.Exec(ctx, `UPDATE tasks SET lease_expires_at = now() - interval '1 second' WHERE id = $1`, taskID)
	if err != nil {
		t.Fatalf("failed to force-expire lease: %v", err)
	}

	n, err := reapExpiredLeases(ctx, pool)
	if err != nil {
		t.Fatalf("reapExpiredLeases failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 task reaped, got %d", n)
	}

	var status string
	var leaseOwner *string
	err = pool.QueryRow(ctx, `SELECT status, lease_owner FROM tasks WHERE id = $1`, taskID).Scan(&status, &leaseOwner)
	if err != nil {
		t.Fatalf("failed to fetch task status: %v", err)
	}
	if status != "READY" {
		t.Fatalf("expected status READY, got %s", status)
	}
	if leaseOwner != nil {
		t.Fatalf("expected lease_owner to be cleared, got %v", *leaseOwner)
	}
}

// TestReapExpiredLeases_MaxAttemptsReached: task RUNNING hai, lease expire ho
// chuki hai, attempt already max_attempts tak pahunch chuka hai -> reaper use
// FAILED kar de (ab retry nahi).
func TestReapExpiredLeases_MaxAttemptsReached(t *testing.T) {
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
		Name: "reaper-maxattempts-workflow-" + uuid.New().String(),
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

	// attempt ko max_attempts tak force karo, aur lease expire kar do
	_, err = pool.Exec(ctx, `UPDATE tasks SET attempt = max_attempts, lease_expires_at = now() - interval '1 second' WHERE id = $1`, taskID)
	if err != nil {
		t.Fatalf("failed to force attempt/expire: %v", err)
	}

	n, err := reapExpiredLeases(ctx, pool)
	if err != nil {
		t.Fatalf("reapExpiredLeases failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 task reaped, got %d", n)
	}

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, taskID).Scan(&status)
	if err != nil {
		t.Fatalf("failed to fetch task status: %v", err)
	}
	if status != "FAILED" {
		t.Fatalf("expected status FAILED, got %s", status)
	}
}
