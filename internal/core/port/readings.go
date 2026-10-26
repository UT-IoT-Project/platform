package port

import (
	"context"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
)

type ReadingsQuery struct {
	Building string
	Room     string
	Start    time.Time
	End      time.Time
	Sensors  []domain.SensorType
}

type ReadingsResult struct {
	Scalars map[domain.SensorType][]domain.ScalarReading
	PIR     []domain.PIRReading
}

type ReadingsService interface {
	GetReadings(ctx context.Context, query ReadingsQuery) (ReadingsResult, error)
}

type ReadingsRepository interface {
	// GetReadings fetches all requested sensors with one database query.
	GetReadings(ctx context.Context, query ReadingsQuery) (ReadingsResult, error)
}
