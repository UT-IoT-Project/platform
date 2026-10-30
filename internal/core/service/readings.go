package service

import (
	"context"
	"errors"

	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

var ErrNoReadings = errors.New("no readings found")

type ReadingsService struct {
	repository port.ReadingsRepository
}

func NewReadingsService(repository port.ReadingsRepository) *ReadingsService {
	return &ReadingsService{repository: repository}
}

func (s *ReadingsService) GetReadings(ctx context.Context, query port.ReadingsQuery) (port.ReadingsResult, error) {
	result, err := s.repository.GetReadings(ctx, query)
	if err != nil {
		return port.ReadingsResult{}, err
	}
	if !hasReadings(result) {
		return port.ReadingsResult{}, ErrNoReadings
	}
	return result, nil
}

func hasReadings(result port.ReadingsResult) bool {
	if len(result.PIR) > 0 {
		return true
	}
	for _, readings := range result.Scalars {
		if len(readings) > 0 {
			return true
		}
	}
	for _, readings := range result.Integers {
		if len(readings) > 0 {
			return true
		}
	}
	return false
}
