package domain

import "time"

type SingleValueReading[T any] struct {
	Timestamp time.Time `json:"timestamp"`
	Value     T         `json:"value"`
}
