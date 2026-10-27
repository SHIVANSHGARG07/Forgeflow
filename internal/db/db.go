// Package db: Postgres connection pool banane ka kaam karta hai.
// Scheduler (aur future me Worker, agar chahiye ho) isi se DB se connect hote hain.
// Connection string yahan hardcode nahi hai - caller (main.go) env variable se deta hai.
package db

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)


func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error){
	pool, err := pgxpool.New(ctx, connString)

	if err != nil{
		return nil, err
	}

	return pool, nil
}