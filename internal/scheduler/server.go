// Package scheduler: Scheduler service ka asli business logic yahan hai.
// `Server` struct `SchedulerServiceServer` (proto se generated interface) ko
// implement karta hai - SubmitWorkflow, PollTask, ReportResult ke functions
// isi file me milenge, jo Postgres pool (s.pool) use karke DB se baat karte hain.
package scheduler

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	schedulerv1 "jobflow/proto/scheduler/v1"
	"time"
)

type Server struct {
	schedulerv1.UnimplementedSchedulerServiceServer
	pool *pgxpool.Pool
}

func NewServer(pool *pgxpool.Pool) *Server {
	return &Server{pool: pool}
}

func (s *Server) SubmitWorkflow(ctx context.Context, req *schedulerv1.SubmitWorkflowRequest) (*schedulerv1.SubmitWorkflowResponse, error) {
	workflowID := uuid.New().String()

	specJSON, err := json.Marshal(req.Tasks)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// db query for workflow table
	_, err = tx.Exec(ctx,
		`INSERT INTO workflows (id, name, spec) VALUES ($1, $2, $3)`,
		workflowID, req.Name, specJSON)

	if err != nil {
		return nil, err
	}

	// db query for workflow_runs table
	runID := uuid.New().String()
	_, err = tx.Exec(ctx,
		`INSERT INTO workflow_runs (id, workflow_id, status, scheduled_for)
		VALUES($1, $2, $3, $4)`,
		runID, workflowID, "RUNNING", time.Now(),
	)

	if err != nil {
		return nil, err
	}

	// tasks
	for _, t := range req.Tasks {
		taskID := uuid.New().String()
		status := "READY"

		if len(t.DependsOn) > 0 {
			status = "PENDING"
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO tasks(id, run_id, name, type, payload, status, idempotency_key)
		VALUES($1, $2, $3, $4, $5, $6, $7)`,
			taskID, runID, t.Name, t.Type, t.Payload, status, runID+":"+t.Name,
		)

		if err != nil {
			return nil, err
		}
	}

	// commit both Exec
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &schedulerv1.SubmitWorkflowResponse{WorkflowId: workflowID}, nil
}


func (s *Server) PollTask(ctx context.Context, req *schedulerv1.PollTaskRequest) (*schedulerv1.PollTaskResponse, error) {

	row := s.pool.QueryRow(ctx, `
		UPDATE tasks
		SET status = 'RUNNING', attempt = attempt + 1, updated_at = now()
		WHERE id = (
			SELECT id from tasks 
			WHERE status = 'READY' AND run_after <= now()
			ORDER BY run_after
			LIMIT 1 
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, type, payload, attempt, idempotency_key
	`)

	var id, taskType, idempotencyKey string
	var payload []byte
	var attempt int32

	err := row.Scan(&id, &taskType, &payload, &attempt, &idempotencyKey)

	if err != nil{
		return &schedulerv1.PollTaskResponse{HasTask: false}, nil
	}

	return &schedulerv1.PollTaskResponse{
		HasTask: true,
		Task: &schedulerv1.Task{
			Id: id,
			Type: taskType,
			Payload: payload,
			Attempt: attempt,
			IdempotencyKey: idempotencyKey,
		},
	}, nil

}