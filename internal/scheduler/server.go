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

	// map task to their uuid
	nameToID := make(map[string]string)
	

	// tasks
	for _, t := range req.Tasks {
		taskID := uuid.New().String()
		nameToID[t.Name] = taskID
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

	// tasks dep
	for _,t := range req.Tasks {
			for _, depName := range t.DependsOn{
				_, err = tx.Exec(ctx,
				`INSERT INTO task_deps (task_id, depends_on_id) VALUES ($1, $2)`,
				nameToID[t.Name], nameToID[depName],
				)

				if err != nil{
					return nil, err
				}
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


func (s *Server) ReportResult(ctx context.Context, req *schedulerv1.ReportResultRequest) (*schedulerv1.ReportResultResponse, error) {

	tx, err := s.pool.Begin(ctx)

	if err != nil{
		return nil, err
	}

	defer tx.Rollback(ctx)

	if req.Success {
		// Case 1: task succeeded
		// - mark this task DONE (with fencing check: WHERE id = ? AND attempt = ?)
		// - find dependent tasks that were PENDING on this task, move them to READY
		//   (only the ones whose ALL dependencies are now DONE)

		tag, err := tx.Exec(ctx,
		`UPDATE tasks SET STATUS = 'DONE' , updated_at = now() WHERE id = $1 AND attempt = $2`,
		req.TaskId, req.Attempt,
		)

		if err != nil{
			return nil, err
		}

		if tag.RowsAffected() == 0{
			return &schedulerv1.ReportResultResponse{Accepted:  false}, nil
		}

		_, err = tx.Exec(ctx,
			`UPDATE tasks t
			SET STATUS = 'READY', run_after = now()
			WHERE t.status = 'PENDING'
			AND t.id in (SELECT task_id FROM task_deps WHERE depends_on_id = $1)
			AND NOT EXISTS (
			SELECT 1 FROM task_deps d JOIN tasks u ON u.id = d.depends_on_id
			WHERE d.task_id = t.id AND u.status <> 'DONE'
		)
			`, req.TaskId)

		if err != nil{
			return nil, err
		}


	} else {
		// Case 2: task failed
		// - if attempt < max_attempts -> set back to READY (retry)
		// - else -> set to FAILED (permanently)

		var attempt, maxAttempts int
		err := tx.QueryRow(ctx,
			`SELECT attempt, max_attempts FROM tasks WHERE id = $1`,
			req.TaskId,
		).Scan(&attempt, &maxAttempts)

		if err != nil {
			return nil, err
		}

		if attempt < maxAttempts {
			// retry
			_, err = tx.Exec(ctx,
				`UPDATE tasks SET status = 'READY' , last_error = $2, updated_at = now()
				WHERE id = $1 AND attempt = $3
				`,
				req.TaskId, req.Error, req.Attempt,
			)
		} else {
			// mark fail
			_, err = tx.Exec(ctx,
				`UPDATE tasks SET status = 'FAILED' , last_error = $2, updated_at = now()
				WHERE id = $1 AND attempt = $3
				`,
				req.TaskId, req.Error, req.Attempt,
			)
		}
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &schedulerv1.ReportResultResponse{Accepted: true}, nil
}