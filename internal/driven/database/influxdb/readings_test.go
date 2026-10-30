package influxdb

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
		{
			Name:     "time",
			Type:     &arrow.TimestampType{Unit: arrow.Nanosecond},
			Nullable: false,
		},
		{
			Name:     "pressure",
			Type:     arrow.PrimitiveTypes.Float64,
			Nullable: true,
		},
		{
			Name:     "humidity",
			Type:     arrow.PrimitiveTypes.Float64,
			Nullable: true,
		},
		{
			Name:     "busy_level",
			Type:     arrow.PrimitiveTypes.Int64,
			Nullable: true,
		},
		{
			Name:     "pir_id",
			Type:     arrow.BinaryTypes.String,
			Nullable: true,
		},
		{
			Name:     "pir_value",
			Type:     arrow.PrimitiveTypes.Float64,
			Nullable: true,
		},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	for _, row := range []struct {
		pressure, humidity *float64
		id                 *string
		pirValue           *float64
	}{
		{
			pressure: ptr(1012.5),
			humidity: ptr(45.0),
		},
		{
			id:       ptr("1"),
			pirValue: ptr(1.0),
		},
		{
			id:       ptr("2"),
			pirValue: ptr(0.0),
		},
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
	if fake.calls != 1 || !strings.Contains(fake.query, "UNION ALL") {
		t.Fatalf("expected one batch query, got calls=%d query=%q", fake.calls, fake.query)
	}
	for _, field := range []string{`"co2Equivalent"`, "busy_level", "activity AS pir_value"} {
		if !strings.Contains(fake.query, field) {
			t.Errorf("query missing firmware field %q: %s", field, fake.query)
		}
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
		PIR: []domain.PIRReading{{Timestamp: stamp, Values: []domain.PIRValue{{ID: "1", Value: 1}, {ID: "2", Value: 0}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestReadingsRepositoryReturnsEmptySeriesForRequestedSensors(t *testing.T) {
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	tests := map[string]struct {
		query             port.ReadingsQuery
		wantResult        port.ReadingsResult
		unwantedMeasure   string
		wantDatabaseCalls int
	}{
		"requested scalar returns an empty series without querying PIR": {
			query: port.ReadingsQuery{
				Building: "B",
				Room:     "R",
				Start:    start,
				End:      start.Add(time.Hour),
				Sensors:  []domain.SensorType{domain.TEMPERATURE_SENSOR},
			},
			wantResult: port.ReadingsResult{
				Scalars: map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {}},
			},
			unwantedMeasure:   "pir_reading",
			wantDatabaseCalls: 1,
		},
		"requested PIR returns an empty series without querying room telemetry": {
			query: port.ReadingsQuery{
				Building: "B",
				Room:     "R",
				Start:    start,
				End:      start.Add(time.Hour),
				Sensors:  []domain.SensorType{domain.PIR_SENSOR},
			},
			wantResult: port.ReadingsResult{
				Scalars: map[domain.SensorType][]domain.ScalarReading{},
				PIR:     []domain.PIRReading{},
			},
			unwantedMeasure:   "room_telemetry",
			wantDatabaseCalls: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fake := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{})}
			repo := &ReadingsRepository{client: fake}
			got, err := repo.GetReadings(context.Background(), tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.wantResult) {
				t.Fatalf("got %#v, want %#v", got, tc.wantResult)
			}
			if fake.calls != tc.wantDatabaseCalls {
				t.Fatalf("database calls = %d, want %d", fake.calls, tc.wantDatabaseCalls)
			}
			if strings.Contains(fake.query, tc.unwantedMeasure) {
				t.Fatalf("unexpected measurement in query: %s", fake.query)
			}
		})
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

// These messages and status codes come from InfluxDB 3.11 Core Flight queries.
func TestReadingsRepositoryHandlesAbsentTelemetrySchema(t *testing.T) {
	for _, tc := range []struct {
		name     string
		failures []error
		sensors  []domain.SensorType
		omitted  string
	}{
		{"room table", []error{status.Error(codes.InvalidArgument, "Error while logically planning query: Error during planning: table 'public.iox.room_telemetry' not found")}, []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR}, "FROM room_telemetry"},
		{"PIR table", []error{status.Error(codes.InvalidArgument, "Error while logically planning query: Error during planning: table 'public.iox.pir_reading' not found")}, []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR}, "FROM pir_reading"},
		{"both tables", []error{status.Error(codes.InvalidArgument, "Error while logically planning query: Error during planning: table 'public.iox.room_telemetry' not found"), status.Error(codes.InvalidArgument, "Error while logically planning query: Error during planning: table 'public.iox.pir_reading' not found")}, []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &schemaFailureClient{failures: tc.failures}
			repo := &ReadingsRepository{client: client}
			got, err := repo.GetReadings(context.Background(), port.ReadingsQuery{Sensors: tc.sensors})
			if err != nil {
				t.Fatal(err)
			}
			for _, sensor := range tc.sensors {
				if sensor == domain.PIR_SENSOR {
					if got.PIR == nil || len(got.PIR) != 0 {
						t.Fatalf("PIR = %#v", got.PIR)
					}
				} else if values, ok := got.Scalars[sensor]; !ok || values == nil || len(values) != 0 {
					t.Fatalf("scalar %v = %#v", sensor, got.Scalars)
				}
			}
			if tc.omitted != "" && strings.Contains(client.queries[len(client.queries)-1], tc.omitted) {
				t.Fatalf("missing schema still queried: %v", client.queries)
			}
			if len(client.queries) > len(tc.failures)+1 {
				t.Fatalf("unbounded retries: %v", client.queries)
			}
		})
	}
}

type schemaFailureClient struct {
	failures []error
	queries  []string
}

func (f *schemaFailureClient) QueryWithParameters(_ context.Context, q string, _ influxdb3.QueryParameters, _ ...influxdb3.QueryOption) (*influxdb3.QueryIterator, error) {
	f.queries = append(f.queries, q)
	if len(f.queries) <= len(f.failures) {
		return nil, f.failures[len(f.queries)-1]
	}
	return influxdb3.NewQueryIteratorFromReader(&oneBatchReader{}), nil
}
func TestReadingsRepositoryDoesNotHideOtherSchemaOrTransportErrors(t *testing.T) {
	for _, err := range []error{
		status.Error(codes.PermissionDenied, "Error while logically planning query: Error during planning: table 'public.iox.room_telemetry' not found"),
		status.Error(codes.InvalidArgument, "Error while logically planning query: Schema error: No field named co2Equivalent. Valid fields are room_telemetry.time."),
		status.Error(codes.InvalidArgument, "Error while logically planning query: Schema error: No field named temperature. Valid fields are room_telemetry.time."),
		status.Error(codes.InvalidArgument, "Error while logically planning query: Error during planning: table 'public.iox.other' not found"),
		errors.New("Error while logically planning query: Error during planning: table 'public.iox.room_telemetry' not found"),
	} {
		client := &schemaFailureClient{failures: []error{err}}
		_, got := (&ReadingsRepository{client: client}).GetReadings(context.Background(), port.ReadingsQuery{Sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.CO2EQUIVALENT_SENSOR}})
		if got == nil || len(client.queries) != 1 {
			t.Fatalf("error %v hidden: %v; queries=%v", err, got, client.queries)
		}
	}
}

func TestReadingsRepositoryQueriesTelegrafScalarFields(t *testing.T) {
	stamp := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: &arrow.TimestampType{Unit: arrow.Nanosecond}},
		{Name: "temperature", Type: arrow.PrimitiveTypes.Float64},
		{Name: "humidity", Type: arrow.PrimitiveTypes.Float64},
		{Name: "iaq", Type: arrow.PrimitiveTypes.Float64},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(stamp.UnixNano()))
	builder.Field(1).(*array.Float64Builder).Append(21.7)
	builder.Field(2).(*array.Float64Builder).Append(45)
	builder.Field(3).(*array.Float64Builder).Append(50)
	record := builder.NewRecordBatch()
	defer record.Release()
	client := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{record: record})}
	repo := &ReadingsRepository{client: client}
	got, err := repo.GetReadings(context.Background(), port.ReadingsQuery{
		Building: "B", Room: "R", Start: stamp.Add(-time.Hour), End: stamp.Add(time.Hour),
		Sensors: []domain.SensorType{domain.TEMPERATURE_SENSOR, domain.HUMIDITY_SENSOR, domain.IAQ_SENSOR},
	})
	if err != nil {
		t.Fatal(err)
	}
	const wantSQL = "SELECT * FROM (SELECT time, temperature, humidity, iaq, NULL AS pir_id, NULL AS pir_value FROM room_telemetry WHERE building = $building AND room = $room AND time >= $start AND time <= $end) AS readings ORDER BY time, pir_id"
	if client.query != wantSQL {
		t.Errorf("SQL = %s, want %s", client.query, wantSQL)
	}
	want := port.ReadingsResult{Scalars: map[domain.SensorType][]domain.ScalarReading{
		domain.TEMPERATURE_SENSOR: {{Timestamp: stamp, Value: 21.7}},
		domain.HUMIDITY_SENSOR:    {{Timestamp: stamp, Value: 45}},
		domain.IAQ_SENSOR:         {{Timestamp: stamp, Value: 50}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readings = %#v, want %#v", got, want)
	}
}

func TestReadingsRepositoryPreservesWirelessCounts(t *testing.T) {
	stamp := time.Date(2026, 10, 26, 10, 0, 0, 0, time.UTC)
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "time", Type: &arrow.TimestampType{Unit: arrow.Nanosecond}},
		{Name: "wifiPackets", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "bleAdverts", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "temperature", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		{Name: "pir_id", Type: arrow.BinaryTypes.String, Nullable: true},
		{Name: "pir_value", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	for i := range 2 {
		builder.Field(0).(*array.TimestampBuilder).Append(arrow.Timestamp(stamp.UnixNano()))
		if i == 0 {
			builder.Field(1).(*array.Int64Builder).Append(4294967295)
			builder.Field(2).(*array.Int64Builder).Append(0)
			builder.Field(3).(*array.Float64Builder).Append(21.7)
			builder.Field(4).AppendNull()
			builder.Field(5).AppendNull()
		} else {
			builder.Field(1).AppendNull()
			builder.Field(2).AppendNull()
			builder.Field(3).AppendNull()
			builder.Field(4).(*array.StringBuilder).Append("1")
			builder.Field(5).(*array.Float64Builder).Append(1)
		}
	}
	record := builder.NewRecordBatch()
	defer record.Release()
	client := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{record: record})}
	got, err := (&ReadingsRepository{client: client}).GetReadings(context.Background(), port.ReadingsQuery{
		Sensors: []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR, domain.TEMPERATURE_SENSOR, domain.PIR_SENSOR, domain.WIFI_PACKETS_SENSOR},
	})
	if err != nil {
		t.Fatal(err)
	}
	const wantSQL = `SELECT * FROM (SELECT time, "wifiPackets", "bleAdverts", temperature, NULL AS pir_id, NULL AS pir_value FROM room_telemetry WHERE building = $building AND room = $room AND time >= $start AND time <= $end UNION ALL SELECT time, NULL AS "wifiPackets", NULL AS "bleAdverts", NULL AS temperature, pir_id, activity AS pir_value FROM pir_reading WHERE building = $building AND room = $room AND time >= $start AND time <= $end) AS readings ORDER BY time, pir_id`
	if client.calls != 1 || client.query != wantSQL {
		t.Fatalf("calls=%d SQL=%s, want one query %s", client.calls, client.query, wantSQL)
	}
	want := port.ReadingsResult{
		Scalars: map[domain.SensorType][]domain.ScalarReading{domain.TEMPERATURE_SENSOR: {{Timestamp: stamp, Value: 21.7}}},
		Integers: map[domain.SensorType][]domain.IntegerReading{
			domain.WIFI_PACKETS_SENSOR:       {{Timestamp: stamp, Value: 4294967295}},
			domain.BLE_ADVERTISEMENTS_SENSOR: {{Timestamp: stamp, Value: 0}},
		},
		PIR: []domain.PIRReading{{Timestamp: stamp, Values: []domain.PIRValue{{ID: "1", Value: 1}}}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readings = %#v, want %#v", got, want)
	}
}

func TestReadingsRepositoryReturnsEmptyWirelessCounts(t *testing.T) {
	client := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{})}
	got, err := (&ReadingsRepository{client: client}).GetReadings(context.Background(), port.ReadingsQuery{Sensors: []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR}})
	if err != nil {
		t.Fatal(err)
	}
	for _, sensor := range []domain.SensorType{domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR} {
		if values, ok := got.Integers[sensor]; !ok || values == nil || len(values) != 0 {
			t.Errorf("integer %v = %#v", sensor, got.Integers)
		}
	}
}

func TestReadingsRepositoryRejectsNonIntegerWirelessCounts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sensor domain.SensorType
	}{
		{"wifiPackets", domain.WIFI_PACKETS_SENSOR}, {"bleAdverts", domain.BLE_ADVERTISEMENTS_SENSOR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := arrow.NewSchema([]arrow.Field{
				{Name: "time", Type: &arrow.TimestampType{Unit: arrow.Nanosecond}},
				{Name: tc.name, Type: arrow.PrimitiveTypes.Float64},
			}, nil)
			builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
			defer builder.Release()
			builder.Field(0).(*array.TimestampBuilder).Append(0)
			builder.Field(1).(*array.Float64Builder).Append(3.0)
			record := builder.NewRecordBatch()
			defer record.Release()
			client := &fakeReadingsClient{rows: influxdb3.NewQueryIteratorFromReader(&oneBatchReader{record: record})}
			_, err := (&ReadingsRepository{client: client}).GetReadings(context.Background(), port.ReadingsQuery{Sensors: []domain.SensorType{tc.sensor}})
			if err == nil || !strings.Contains(err.Error(), tc.name) || !strings.Contains(err.Error(), "float64") {
				t.Fatalf("error = %v, want field-specific float64 type error", err)
			}
		})
	}
}

func TestReadingsRepositoryDoesNotHideMissingWirelessCountColumns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		sensor domain.SensorType
	}{
		{"wifiPackets", domain.WIFI_PACKETS_SENSOR}, {"bleAdverts", domain.BLE_ADVERTISEMENTS_SENSOR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failure := status.Error(codes.InvalidArgument, "Error while logically planning query: Schema error: No field named "+tc.name+". Valid fields are room_telemetry.time.")
			client := &schemaFailureClient{failures: []error{failure}}
			_, err := (&ReadingsRepository{client: client}).GetReadings(context.Background(), port.ReadingsQuery{Sensors: []domain.SensorType{tc.sensor}})
			if !errors.Is(err, failure) || len(client.queries) != 1 {
				t.Fatalf("error = %v, calls = %d; want original error after one query", err, len(client.queries))
			}
		})
	}
}
