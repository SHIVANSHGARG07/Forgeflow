package executors

// dependencies
import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// make sql payload struct
type SQLPayload struct {
	Query string `json:"query"`
}

// RunSQL: payload parse karke query ko pool ke through chalata hai.
func RunSQL(ctx context.Context, payload []byte, pool *pgxpool.Pool) error {
	var p SQLPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid sql payload: %w", err)
	}

	if p.Query == "" {
		return fmt.Errorf("sql payload missing query")
	}

	if _, err := pool.Exec(ctx, p.Query); err != nil {
		return fmt.Errorf("sql query failed: %w", err)
	}

	return nil
}
