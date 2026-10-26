package domain

import "time"

type MultipleValueReading[T any] struct {
	Timestamp time.Time `json:"timestamp"`
	Values    []struct {
		ID    string `json:"id"`
		Value T      `json:"value"`
	} `json:"values"`
}
