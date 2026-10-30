package response

import (
	"fmt"
	"math"
	"time"

	"github.com/jouleion/busyness-sensing/backend/internal/core/domain"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

type Readings struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}
type scalarReadingResponse struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}
type integerReadingResponse struct {
	Timestamp string `json:"timestamp"`
	Value     int64  `json:"value"`
}
type busyLevelResponse struct {
	Timestamp string `json:"timestamp"`
	Value     int8   `json:"value"`
}
type pirValueResponse struct {
	ID    string  `json:"ID"`
	Value float64 `json:"value"`
}
type pirReadingResponse struct {
	Timestamp string             `json:"timestamp"`
	Values    []pirValueResponse `json:"values"`
}

func MapReadings(query port.ReadingsQuery, result port.ReadingsResult) (Readings, error) {
	location, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		return Readings{}, err
	}
	formatTime := func(timestamp time.Time) string { return timestamp.In(location).Format("2006-01-02 15:04:05.000") }
	names := map[domain.SensorType]string{
		domain.PIR_SENSOR: "PIRs", domain.PRESSURE_SENSOR: "pressure", domain.HUMIDITY_SENSOR: "humidity", domain.TEMPERATURE_SENSOR: "temperature",
		domain.VOC_SENSOR: "voc", domain.IAQ_SENSOR: "iaq", domain.MIC_VOL_SENSOR: "mic_vol",
		domain.WIFI_PACKETS_SENSOR: "wifiPackets", domain.BLE_ADVERTISEMENTS_SENSOR: "bleAdvertisements",
		domain.CO2EQUIVALENT_SENSOR: "co2equivalent", domain.BUSY_LEVEL: "busy_level",
	}
	response := Readings{Status: 200, Message: "data retrieved successfully", Data: make(map[string]any)}
	for _, sensor := range query.Sensors {
		name, ok := names[sensor]
		if !ok {
			return Readings{}, fmt.Errorf("unsupported sensor %d", sensor)
		}
		if _, exists := response.Data[name]; exists {
			continue
		}
		switch sensor {
		case domain.PIR_SENSOR:
			readings := make([]pirReadingResponse, 0, len(result.PIR))
			for _, reading := range result.PIR {
				values := make([]pirValueResponse, 0, len(reading.Values))
				for _, value := range reading.Values {
					if !finite(value.Value) {
						return Readings{}, fmt.Errorf("invalid PIR value")
					}
					values = append(values, pirValueResponse{ID: value.ID, Value: value.Value})
				}
				readings = append(readings, pirReadingResponse{Timestamp: formatTime(reading.Timestamp), Values: values})
			}
			response.Data[name] = readings
		case domain.WIFI_PACKETS_SENSOR, domain.BLE_ADVERTISEMENTS_SENSOR:
			readings := make([]integerReadingResponse, 0, len(result.Integers[sensor]))
			for _, reading := range result.Integers[sensor] {
				readings = append(readings, integerReadingResponse{Timestamp: formatTime(reading.Timestamp), Value: reading.Value})
			}
			response.Data[name] = readings
		case domain.BUSY_LEVEL:
			readings := make([]busyLevelResponse, 0, len(result.Scalars[sensor]))
			for _, reading := range result.Scalars[sensor] {
				if !finite(reading.Value) || reading.Value < -128 || reading.Value > 127 || math.Trunc(reading.Value) != reading.Value {
					return Readings{}, fmt.Errorf("invalid busy level")
				}
				readings = append(readings, busyLevelResponse{Timestamp: formatTime(reading.Timestamp), Value: int8(reading.Value)})
			}
			response.Data[name] = readings
		default:
			readings := make([]scalarReadingResponse, 0, len(result.Scalars[sensor]))
			for _, reading := range result.Scalars[sensor] {
				if !finite(reading.Value) {
					return Readings{}, fmt.Errorf("invalid scalar value")
				}
				readings = append(readings, scalarReadingResponse{Timestamp: formatTime(reading.Timestamp), Value: reading.Value})
			}
			response.Data[name] = readings
		}
	}
	return response, nil
}
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
