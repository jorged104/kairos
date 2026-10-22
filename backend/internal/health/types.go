package health

import (
	"context"
	"time"
)

// DatabaseTimeout is how long the readiness check waits for the database.
const DatabaseTimeout = 2 * time.Second

// Pinger checks that the database answers.
type Pinger interface {
	Ping(ctx context.Context) error
}

type statusResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}
