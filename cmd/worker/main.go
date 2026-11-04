// Step 1: Scheduler se gRPC connection banao
// Step 2: ek client banao (jisse PollTask/ReportResult call kar sako)
// Step 3: infinite loop:
//    - PollTask call karo
//    - agar task mila -> "kaam karo" (sleep type ke liye time.Sleep)
//    - ReportResult call karo (success/failure)
//    - agar task nahi mila -> thoda wait karke dobara try karo

package main

import (
	"context"
	"fmt"
	"jobflow/internal/db"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"jobflow/internal/worker/executors"
	schedulerv1 "jobflow/proto/scheduler/v1"
)

func main() {

	schedulerAddr := os.Getenv("SCHEDULER_ADDR")
	if schedulerAddr == "" {
		schedulerAddr = "localhost:50051"
	}

	worker_client_conn, err := grpc.NewClient(schedulerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatalf("failed to connect to scheduler: %v", err)
	}

	defer worker_client_conn.Close()

	log.Printf("connected to scheduler at %s", schedulerAddr)

	// worker starts working with scheduler
	client := schedulerv1.NewSchedulerServiceClient(worker_client_conn)

	ctx := context.Background()

	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	pool, err := db.NewPool(ctx, connString)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// call PollTask from worker to scheduler, bar bar (infinite loop)
	for {
		resp, err := client.PollTask(ctx, &schedulerv1.PollTaskRequest{WorkerId: "worker-1"})
		if err != nil {
			log.Printf("PollTask failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if !resp.HasTask {
			log.Println("no task available, waiting...")
			time.Sleep(2 * time.Second)
			continue
		}

		log.Printf("got task: %s", resp.Task.Id)

		heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
		go sendHeartbeats(heartbeatCtx, client, "worker-1", resp.Task.Id, resp.Task.Attempt)

		var taskErr error

		// start actual working
		switch resp.Task.Type {
		case "http":
			taskErr = executors.RunHTTP(resp.Task.Payload, resp.Task.IdempotencyKey)
		case "shell":
			taskErr = executors.RunShell(resp.Task.Payload)
		case "sql":
			taskErr = executors.RunSQL(ctx, resp.Task.Payload, pool)
		default:
			taskErr = fmt.Errorf("unknown task type: %s", resp.Task.Type)
		}

		cancelHeartbeat()


		if taskErr != nil {
			log.Printf("task %s failed: %v", resp.Task.Id, taskErr)
		} else {
			log.Printf("task %s done, reporting result", resp.Task.Id)
		}

		errMsg := ""
		if taskErr != nil {
			errMsg = taskErr.Error()
		}

		_, err = client.ReportResult(ctx, &schedulerv1.ReportResultRequest{
			WorkerId: "worker-1",
			TaskId:   resp.Task.Id,
			Attempt:  resp.Task.Attempt,
			Success:  taskErr == nil,
			Error:    errMsg,
		})
		if err != nil {
			log.Printf("ReportResult failed: %v", err)
		}
	}
}


func sendHeartbeats(ctx context.Context, client schedulerv1.SchedulerServiceClient, workerID, taskID string, attempt int32) {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            client.HeartBeat(ctx, &schedulerv1.HeartbeatRequest{
                WorkerId: workerID,
                TaskId:   taskID,
                Attempt:  attempt,
            })
        }
    }
}