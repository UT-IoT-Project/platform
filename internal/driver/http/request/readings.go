package request

import (
	"errors"
	"time"
	_ "time/tzdata" // Keep Amsterdam time conversion available in minimal runtime images.

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

var (
	ErrInvalidTimestamp = errors.New("invalid timestamp")
	ErrEmptySensors     = errors.New("empty sensors")
	ErrUnknownSensor    = errors.New("unknown sensor")
)

const timestampLayout = "2006-01-02 15:04:05.000"

var sensorTypes = map[string]domain.SensorType{
	"PIRs":          domain.PIR_SENSOR,
	"pressure":      domain.PRESSURE_SENSOR,
	"humidity":      domain.HUMIDITY_SENSOR,
	"temperature":   domain.TEMPERATURE_SENSOR,
	"co2equivalent": domain.CO2EQUIVALENT_SENSOR,
	"voc":           domain.VOC_SENSOR,
	"iaq":           domain.IAQ_SENSOR,
	"mic_vol":       domain.MIC_VOL_SENSOR,
	"busy_level":    domain.BUSY_LEVEL,
}

type Readings struct {
	Building   string   `param:"building"`
	RoomNumber string   `param:"roomNumber"`
	StartTime  string   `query:"start_time"`
	EndTime    string   `query:"end_time"`
	Sensors    []string `query:"sensors"`
}

func (r Readings) ToQuery() (port.ReadingsQuery, error) {
	location, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		return port.ReadingsQuery{}, err
	}
	start, err := parseTimestamp(r.StartTime, location)
	if err != nil {
		return port.ReadingsQuery{}, err
	}
	end, err := parseTimestamp(r.EndTime, location)
	if err != nil {
		return port.ReadingsQuery{}, err
	}
	if len(r.Sensors) == 0 {
		return port.ReadingsQuery{}, ErrEmptySensors
	}

	sensors := make([]domain.SensorType, 0, len(r.Sensors))
	for _, name := range r.Sensors {
		if name == "" {
			return port.ReadingsQuery{}, ErrEmptySensors
		}
		sensor, ok := sensorTypes[name]
		if !ok {
			return port.ReadingsQuery{}, ErrUnknownSensor
		}
		sensors = append(sensors, sensor)
	}
	return port.ReadingsQuery{
		Building: r.Building,
		Room:     r.RoomNumber,
		Start:    start.UTC(),
		End:      end.UTC(),
		Sensors:  sensors,
	}, nil
}

func parseTimestamp(value string, location *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation(timestampLayout, value, location)
	if err != nil || parsed.In(location).Format(timestampLayout) != value {
		return time.Time{}, ErrInvalidTimestamp
	}
	// Reject repeated wall times by checking the offsets around a DST transition.
	_, currentOffset := parsed.In(location).Zone()
	for _, nearby := range []time.Time{parsed.Add(-24 * time.Hour), parsed.Add(24 * time.Hour)} {
		_, otherOffset := nearby.In(location).Zone()
		if otherOffset == currentOffset {
			continue
		}
		alternative := parsed.Add(time.Duration(currentOffset-otherOffset) * time.Second)
		if alternative.In(location).Format(timestampLayout) == value {
			return time.Time{}, ErrInvalidTimestamp
		}
	}
	return parsed, nil
}
