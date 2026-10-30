package influxdb

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

type fakeReadingsClient struct {
	query  string
	params influxdb3.QueryParameters
	calls  int
	rows   *influxdb3.QueryIterator
	err    error
}

func (f *fakeReadingsClient) QueryWithParameters(_ context.Context, query string, params influxdb3.QueryParameters, _ ...influxdb3.QueryOption) (*influxdb3.QueryIterator, error) {
	f.calls++
	f.query, f.params = query, params
	return f.rows, f.err
}

type oneBatchReader struct {
	record arrow.RecordBatch
	read   bool
	err    error
}

func (r *oneBatchReader) Next() bool {
	if r.read || r.record == nil {
		return false
	}
	r.read = true
	return true
}
func (r *oneBatchReader) RecordBatch() arrow.RecordBatch { return r.record }
func (r *oneBatchReader) Err() error                     { return r.err }
func (r *oneBatchReader) Schema() *arrow.Schema          { return r.record.Schema() }

func TestReadingsRepositoryCombinesScalarAndPIRRows(t *testing.T) {
	stamp := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: &arrow.TimestampType{Unit: arrow.Nanosecond}, Nullable: false},
		{Name: "pressure", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "hum", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "busy_level", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "pir_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "pir_value", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	for _, row := range []struct {
		pressure, humidity *float64
		id                 *string
		pirValue           *float64
	}{
		{pressure: ptr(1012.5), humidity: ptr(45.0)},
		{id: ptr("north"), pirValue: ptr(1.0)},
		{id: ptr("south"), pirValue: ptr(0.0)},
	} {
		builder.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(stamp.UnixNano()))
		appendFloat(builder.Field(1).(*array.Float64Builder), row.pressure)
		appendFloat(builder.Field(2).(*array.Float64Builder), row.humidity)
		if row.pressure == nil {
			builder.Field(3).(*array.Int64Builder).AppendNull()
		} else {
			builder.Field(3).(*array.Int64Builder).Append(3)
		}
		if row.id == nil {
			builder.Field(4).(*array.StringBuilder).AppendNull()
		} else {
			builder.Field(4).(*array.StringBuilder).Append(*row.id)
		}
		appendFloat(builder.Field(5).(*array.Float64Builder), row.pirValue)
	}
	record := builder.NewRecordBatch()
	defer record.Release()
	fake := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{record: record})}
	repo := &ReadingsRepository{client: fake}
	query := port.ReadingsQuery{Building: "Zilverling' OR 1=1 --", Room: "Z2402", Start: stamp.Add(-time.Hour), End: stamp.Add(time.Hour), Sensors: []domain.SensorType{domain.PRESSURE_SENSOR, domain.HUMIDITY_SENSOR, domain.BUSY_LEVEL, domain.PIR_SENSOR, domain.CO2EQUIVALENT_SENSOR}}

	got, err := repo.GetReadings(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 || !strings.Contains(fake.query, "UNION ALL") || strings.Contains(strings.ToUpper(fake.query), "LIMIT") {
		t.Fatalf("expected one batch query without a row cap, got calls=%d query=%q", fake.calls, fake.query)
	}
	if strings.Contains(fake.query, query.Building) || fake.params["building"] != query.Building || fake.params["room"] != query.Room {
		t.Fatalf("location must be parameterized: query=%q params=%v", fake.query, fake.params)
	}
	if fake.params["start"] != "2026-10-26T09:00:00Z" || fake.params["end"] != "2026-10-26T11:00:00Z" {
		t.Fatalf("unexpected UTC time bounds: %v", fake.params)
	}
	want := port.ReadingsResult{
		Scalars: map[domain.SensorType][]domain.ScalarReading{
			domain.PRESSURE_SENSOR:      {{Timestamp: stamp, Value: 1012.5}},
			domain.HUMIDITY_SENSOR:      {{Timestamp: stamp, Value: 45}},
			domain.BUSY_LEVEL:           {{Timestamp: stamp, Value: 3}},
			domain.CO2EQUIVALENT_SENSOR: {},
		},
		PIR: []domain.PIRReading{{Timestamp: stamp, Values: []domain.PIRValue{{ID: "north", Value: 1}, {ID: "south", Value: 0}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestReadingsRepositoryReturnsMoreThanTenThousandRows(t *testing.T) {
	const rowCount = 10001
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: &arrow.TimestampType{Unit: arrow.Nanosecond}},
		{Name: "temp", Type: arrow.PrimitiveTypes.Float64},
		{Name: "pir_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "pir_value", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	for i := range rowCount {
		builder.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(start.Add(time.Duration(i) * time.Second).UnixNano()))
		builder.Field(1).(*array.Float64Builder).Append(float64(i))
		builder.Field(2).(*array.StringBuilder).AppendNull()
		builder.Field(3).(*array.Float64Builder).AppendNull()
	}
	record := builder.NewRecordBatch()
	defer record.Release()
	fake := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{record: record})}
	repo := &ReadingsRepository{client: fake}

	got, err := repo.GetReadings(context.Background(), port.ReadingsQuery{
		Building: "B", Room: "R", Start: start, End: start.Add(rowCount * time.Second),
		Sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR},
	})
	if err != nil {
		t.Fatal(err)
	}
	series := got.Scalars[domain.TEMPERATURE_SENSOR]
	if len(series) != rowCount || series[rowCount-1].Value != rowCount-1 {
		t.Fatalf("expected all %d readings, got %d", rowCount, len(series))
	}
	if strings.Contains(strings.ToUpper(fake.query), "LIMIT") {
		t.Fatalf("query still has a row cap: %s", fake.query)
	}
}

func TestReadingsRepositoryReturnsEmptySeriesForRequestedSensors(t *testing.T) {
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name            string
		sensors         []domain.SensorType
		unwantedMeasure string
		want            port.ReadingsResult
	}{
		{name: "scalar", sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR}, unwantedMeasure: "pir_reading", want: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {}}}},
		{name: "PIR", sensors: []domain.SensorType{domain.PIR_SENSOR}, unwantedMeasure: "room_telemetry", want: port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{}, PIR: []domain.PIRReading{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{})}
			repo := &ReadingsRepository{client: fake}
			got, err := repo.GetReadings(context.Background(), port.ReadingsQuery{Building: "B", Room: "R", Start: start, End: start.Add(time.Hour), Sensors: tc.sensors})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
			if strings.Contains(fake.query, tc.unwantedMeasure) {
				t.Fatalf("unexpected measurement in query: %s", fake.query)
			}
		})
	}
}

func TestReadingsRepositoryLeavesRequestValidationToCaller(t *testing.T) {
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		query port.ReadingsQuery
	}{
		{name: "empty building", query: port.ReadingsQuery{Room: "R", Start: start, End: start.Add(time.Hour), Sensors: []domain.SensorType{domain.PIR_SENSOR}}},
		{name: "empty room", query: port.ReadingsQuery{Building: "B", Start: start, End: start.Add(time.Hour), Sensors: []domain.SensorType{domain.PIR_SENSOR}}},
		{name: "zero start", query: port.ReadingsQuery{Building: "B", Room: "R", End: start.Add(time.Hour), Sensors: []domain.SensorType{domain.PIR_SENSOR}}},
		{name: "reversed range", query: port.ReadingsQuery{Building: "B", Room: "R", Start: start.Add(time.Hour), End: start, Sensors: []domain.SensorType{domain.PIR_SENSOR}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{})}
			repo := &ReadingsRepository{client: fake}
			if _, err := repo.GetReadings(context.Background(), tc.query); err != nil {
				t.Fatal(err)
			}
			if fake.calls != 1 {
				t.Fatalf("expected one database query, got %d", fake.calls)
			}
		})
	}
}

func TestReadingsRepositoryRejectsUnsupportedSensorType(t *testing.T) {
	fake := &fakeReadingsClient{}
	repo := &ReadingsRepository{client: fake}
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	query := port.ReadingsQuery{Building: "B", Room: "R", Start: start, End: start.Add(time.Hour), Sensors: []domain.SensorType{domain.SensorType(100)}}
	if _, err := repo.GetReadings(context.Background(), query); err == nil || !strings.Contains(err.Error(), "unsupported sensor type") {
		t.Fatalf("expected unsupported sensor error, got %v", err)
	}
	if fake.calls != 0 {
		t.Fatalf("unsupported sensor reached database %d times", fake.calls)
	}
}

func TestReadingsRepositoryPropagatesDatabaseError(t *testing.T) {
	fake := &fakeReadingsClient{err: errors.New("query failed")}
	repo := &ReadingsRepository{client: fake}
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	_, err := repo.GetReadings(context.Background(), port.ReadingsQuery{Building: "B", Room: "R", Start: start, End: start.Add(time.Hour), Sensors: []domain.SensorType{domain.PIR_SENSOR}})
	if err == nil || !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("expected database error, got %v", err)
	}
}

func ptr[T any](value T) *T { return &value }

func appendFloat(builder *array.Float64Builder, value *float64) {
	if value == nil {
		builder.AppendNull()
	} else {
		builder.Append(*value)
	}
}
