// cmd/scheduler/main.go: Scheduler service ka entry point.
// Kaam: DATABASE_URL env variable padhna, Postgres connection pool banana,
// (aage) gRPC server start karna jo internal/scheduler.Server ko expose karega.
package main

import (
	"context"
	"log"
	"os"
	"net"

	"google.golang.org/grpc"
	"jobflow/internal/db"
	"jobflow/internal/scheduler"
	schedulerv1 "jobflow/proto/scheduler/v1"

)


func main() {

	connString := os.Getenv("DATABASE_URL")
	if connString == ""{
		log.Fatal("DATABASE_URL environment variable is not set")
	}


	ctx := context.Background()
	pool, err := db.NewPool(ctx, connString)

	if err != nil{
		log.Fatalf("failed to connect to database: %v", err)
	}	
	defer pool.Close()

	log.Println("connected to database successfully")


	// run actual grpc server for scheduler
	listener, err := net.Listen("tcp", ":50051")

	if err != nil{
		log.Fatalf("Failed to listen : %v", err)
	}

	grpcServer := grpc.NewServer()
	schedulerv1.RegisterSchedulerServiceServer(grpcServer, scheduler.NewServer(pool))

	log.Println("scheduler gRPC server listening on : 50051")

	if err := grpcServer.Serve(listener); err != nil{
		log.Fatalf("failed to serve: %v", err)
	}


}