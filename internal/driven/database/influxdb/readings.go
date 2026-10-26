package influxdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

const maxReadingRows = 10000

var scalarFields = map[domain.SensorType]string{
	domain.PRESSURE_SENSOR:      "pressure",
	domain.HUMIDITY_SENSOR:      "hum",
	domain.TEMPERATURE_SENSOR:   "temp",
	domain.VOC_SENSOR:           "voc",
	domain.AIRRESISTANCE_SENSOR: "airresistance",
	domain.MIC_VOL_SENSOR:       "mic_vol",
	domain.CO2EQUIVALENT_SENSOR: "co2equivalent",
	domain.BUSY_LEVEL:           "busy_level",
}

type readingsQueryClient interface {
	QueryWithParameters(context.Context, string, influxdb3.QueryParameters, ...influxdb3.QueryOption) (*influxdb3.QueryIterator, error)
}

// ReadingsRepository queries the Telegraf room and PIR measurements.
type ReadingsRepository struct {
	client readingsQueryClient
}

var _ port.ReadingsRepository = (*ReadingsRepository)(nil)

func NewReadingsRepository(client *influxdb3.Client) *ReadingsRepository {
	if client == nil {
		return &ReadingsRepository{}
	}
	return &ReadingsRepository{client: client}
}

func (r *ReadingsRepository) GetReadings(ctx context.Context, query port.ReadingsQuery) (port.ReadingsResult, error) {
	result := port.ReadingsResult{Scalars: make(map[domain.SensorType][]domain.ScalarReading)}
	if r == nil || r.client == nil {
		return result, errors.New("influxdb readings client is nil")
	}
	if query.Building == "" || query.Room == "" || query.Start.IsZero() || !query.Start.Before(query.End) || len(query.Sensors) == 0 {
		return result, errors.New("invalid readings query")
	}

	fields := make([]string, 0, len(query.Sensors))
	wantPIR := false
	for _, sensor := range query.Sensors {
		if sensor == domain.PIR_SENSOR {
			wantPIR = true
			continue
		}
		field, ok := scalarFields[sensor]
		if !ok {
			return result, fmt.Errorf("unsupported sensor type %d", sensor)
		}
		if _, seen := result.Scalars[sensor]; !seen {
			result.Scalars[sensor] = []domain.ScalarReading{}
			fields = append(fields, field)
		}
	}
	if wantPIR {
		result.PIR = []domain.PIRReading{}
	}

	statement := readingsSQL(fields, wantPIR)
	params := influxdb3.QueryParameters{
		"building": query.Building,
		"room":     query.Room,
		"start":    query.Start.UTC().Format(time.RFC3339Nano),
		"end":      query.End.UTC().Format(time.RFC3339Nano),
	}
	queryContext, cancel := context.WithCancel(ctx)
	defer cancel()
	rows, err := r.client.QueryWithParameters(queryContext, statement, params)
	if err != nil {
		return result, fmt.Errorf("query influxdb readings: %w", err)
	}
	if rows == nil {
		return result, errors.New("query influxdb readings: nil result")
	}

	pirIndex := make(map[time.Time]int)
	count := 0
	for rows.Next() {
		count++
		if count > maxReadingRows {
			return port.ReadingsResult{}, fmt.Errorf("readings query exceeds %d rows", maxReadingRows)
		}
		row := rows.Value()
		stamp, err := readingTime(row["time"])
		if err != nil {
			return port.ReadingsResult{}, err
		}
		for sensor, field := range scalarFields {
			series, requested := result.Scalars[sensor]
			if !requested || row[field] == nil {
				continue
			}
			value, err := readingValue(row[field])
			if err != nil {
				return port.ReadingsResult{}, fmt.Errorf("invalid %s reading: %w", field, err)
			}
			result.Scalars[sensor] = append(series, domain.ScalarReading{Timestamp: stamp, Value: value})
		}
		if wantPIR && row["pir_id"] != nil {
			id, ok := row["pir_id"].(string)
			if !ok || id == "" {
				return port.ReadingsResult{}, errors.New("invalid PIR sensor ID")
			}
			value, err := readingValue(row["pir_value"])
			if err != nil {
				return port.ReadingsResult{}, fmt.Errorf("invalid PIR reading: %w", err)
			}
			index, found := pirIndex[stamp]
			if !found {
				index = len(result.PIR)
				pirIndex[stamp] = index
				result.PIR = append(result.PIR, domain.PIRReading{Timestamp: stamp, Values: []domain.PIRValue{}})
			}
			result.PIR[index].Values = append(result.PIR[index].Values, domain.PIRValue{ID: id, Value: value})
		}
	}
	if err := rows.Err(); err != nil {
		return port.ReadingsResult{}, fmt.Errorf("read influxdb readings: %w", err)
	}
	return result, nil
}

func readingsSQL(fields []string, wantPIR bool) string {
	parts := make([]string, 0, 2)
	if len(fields) > 0 {
		selects := append([]string{"time"}, fields...)
		selects = append(selects, "NULL AS pir_id", "NULL AS pir_value")
		parts = append(parts, "SELECT "+strings.Join(selects, ", ")+" FROM room_telemetry WHERE building = $building AND room = $room AND time >= $start AND time <= $end")
	}
	if wantPIR {
		selects := []string{"time"}
		for _, field := range fields {
			selects = append(selects, "NULL AS "+field)
		}
		selects = append(selects, "pir_id", "value AS pir_value")
		parts = append(parts, "SELECT "+strings.Join(selects, ", ")+" FROM pir_reading WHERE building = $building AND room = $room AND time >= $start AND time <= $end")
	}
	return "SELECT * FROM (" + strings.Join(parts, " UNION ALL ") + ") AS readings ORDER BY time, pir_id LIMIT 10001"
}

func readingTime(value any) (time.Time, error) {
	switch value := value.(type) {
	case time.Time:
		return value.UTC(), nil
	case arrow.Timestamp:
		return value.ToTime(arrow.Nanosecond).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("invalid reading timestamp %T", value)
	}
}

func readingValue(value any) (float64, error) {
	switch value := value.(type) {
	case float64:
		return value, nil
	case float32:
		return float64(value), nil
	case int64:
		return float64(value), nil
	default:
		return 0, fmt.Errorf("unexpected value type %T", value)
	}
}
