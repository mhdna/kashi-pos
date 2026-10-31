package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Store struct {
	*Queries
	conn *sql.DB
}

func NewStore(conn *sql.DB) *Store {
	return &Store{
		Queries: New(conn),
		conn:    conn,
	}
}

func (s *Store) ExecTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	q := s.WithTx(tx)
	if err := fn(q); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("tx error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	return tx.Commit()
}
