package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
	"github.com/jouleion/busyness-sensing/backend/internal/core/service"
)

type readingsRepositoryFunc func(context.Context, port.ReadingsQuery) (port.ReadingsResult, error)

func (f readingsRepositoryFunc) GetReadings(ctx context.Context, query port.ReadingsQuery) (port.ReadingsResult, error) {
	return f(ctx, query)
}

func TestReadingsServiceGetReadings(t *testing.T) {
	stamp := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	query := port.ReadingsQuery{Building: "Zilverling", Room: "101", Start: stamp, End: stamp.Add(time.Hour), Sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repositoryError := errors.New("database unavailable")
	scalars := map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {{Timestamp: stamp, Value: 21.5}}}
	pir := []domain.PIRReading{{Timestamp: stamp, Values: []domain.PIRValue{{ID: "door", Value: 1}}}}
	tests := []struct {
		name    string
		result  port.ReadingsResult
		err     error
		wantErr error
	}{
		{name: "populated", result: port.ReadingsResult{Scalars: scalars, PIR: pir}},
		{name: "scalar only partial", result: port.ReadingsResult{Scalars: scalars}},
		{name: "PIR only partial", result: port.ReadingsResult{PIR: pir}},
		{name: "nil results", wantErr: service.ErrNoReadings},
		{name: "empty collections", result: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {}}, PIR: []domain.PIRReading{}}, wantErr: service.ErrNoReadings},
		{name: "repository failure", err: repositoryError, wantErr: repositoryError},
		{name: "repository failure with data", result: port.ReadingsResult{Scalars: scalars}, err: repositoryError, wantErr: repositoryError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			repo := readingsRepositoryFunc(func(gotCtx context.Context, gotQuery port.ReadingsQuery) (port.ReadingsResult, error) {
				calls++
				if gotCtx != ctx || !reflect.DeepEqual(gotQuery, query) {
					t.Errorf("repository received context %v and query %+v; want original context and query %+v", gotCtx, gotQuery, query)
				}
				return tt.result, tt.err
			})
			got, err := service.NewReadingsService(repo).GetReadings(ctx, query)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("GetReadings error = %v, want %v", err, tt.wantErr)
			}
			if calls != 1 {
				t.Errorf("repository called %d times, want 1", calls)
			}
			if err == nil && !reflect.DeepEqual(got, tt.result) {
				t.Errorf("GetReadings result = %+v, want %+v", got, tt.result)
			}
		})
	}
}
