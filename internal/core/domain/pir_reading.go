package domain

import "time"

type PIRValue struct {
	ID    string
	Value float64
}

type PIRReading struct {
	Timestamp time.Time
	Values    []PIRValue
}
