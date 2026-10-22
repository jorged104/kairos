package health

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	db      Pinger
	timeout time.Duration
}

func NewService(db Pinger, timeout time.Duration) *Service {
	return &Service{db: db, timeout: timeout}
}

// DatabaseReady returns an error if the database does not answer within the timeout.
func (s *Service) DatabaseReady(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
