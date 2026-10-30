package domain

import "time"

type IntegerReading struct {
	Timestamp time.Time
	Value     int64
}
