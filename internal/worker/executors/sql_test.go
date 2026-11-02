package executors

// dependencies
import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunSQL ko real Postgres chahiye (integration test), isliye DATABASE_URL
// env variable se pool banega. Agar set nahi hai, test skip ho jayega.

func TestRunSQL_Success(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	err = RunSQL(context.Background(), []byte(`{"query": "SELECT 1"}`), pool)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunSQL_MissingQuery(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	err = RunSQL(context.Background(), []byte(`{"query": ""}`), pool)
	if err == nil {
		t.Fatal("expected error for missing query, got nil")
	}
}

func TestRunSQL_InvalidQuery(t *testing.T) {
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	err = RunSQL(context.Background(), []byte(`{"query": "SELECT * FROM nonexistent_table_xyz"}`), pool)
	if err == nil {
		t.Fatal("expected error for invalid query, got nil")
	}
}
