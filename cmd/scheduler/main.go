// cmd/scheduler/main.go: Scheduler service ka entry point.
// Kaam: DATABASE_URL env variable padhna, Postgres connection pool banana,
// (aage) gRPC server start karna jo internal/scheduler.Server ko expose karega.
package main

import (
	"context"
	"log"
	"os"

	"jobflow/internal/db"
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


}