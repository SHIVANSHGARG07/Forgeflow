package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// reapExpiredLeases: ek hi pass me saare "RUNNING" tasks dhundta hai jinki
// lease expire ho chuki hai (matlab worker crash ho gaya, heartbeat band ho
// gayi). Agar attempt abhi max_attempts se kam hai -> READY kar deta hai
// (retry ke liye), warna FAILED kar deta hai (bahut retries ho chuke).
func reapExpiredLeases(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, `
		UPDATE tasks
		SET status = CASE WHEN attempt >= max_attempts THEN 'FAILED' ELSE 'READY' END,
		    lease_owner = NULL,
		    lease_expires_at = NULL,
		    updated_at = now()
		WHERE status = 'RUNNING' AND lease_expires_at < now()
	`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// StartReaper: background loop, har 15 second ek baar reapExpiredLeases
// chalata hai, jab tak scheduler process zinda hai (infinite loop).
func StartReaper(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := reapExpiredLeases(ctx, pool)
			if err != nil {
				log.Printf("reaper: failed to reap expired leases: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("reaper: reaped %d expired task lease(s)", n)
			}
		}
	}
}
