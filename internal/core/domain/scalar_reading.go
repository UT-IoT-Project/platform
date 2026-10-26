package domain

import "time"

type ScalarReading struct {
	Timestamp time.Time
	Value     float64
}
