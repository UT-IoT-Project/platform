package http

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/jouleion/busyness-sensing/backend/internal/core/service"
	"github.com/jouleion/busyness-sensing/backend/internal/driver/http/request"
	"github.com/jouleion/busyness-sensing/backend/internal/driver/http/response"
)

type errorResponse struct {
	Status  int            `json:"status"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data"`
}

func (s *Server) getReadings(c *echo.Context) error {
	var input request.Readings
	if err := echo.BindPathValues(c, &input); err != nil {
		return readingsError(c, http.StatusBadRequest, "Invalid timestamp format.")
	}
	if err := echo.BindQueryParams(c, &input); err != nil {
		return readingsError(c, http.StatusBadRequest, "Invalid timestamp format.")
	}
	query, err := input.ToQuery()
	if err != nil {
		switch {
		case errors.Is(err, request.ErrInvalidTimestamp):
			return readingsError(c, http.StatusBadRequest, "Invalid timestamp format.")
		case errors.Is(err, request.ErrEmptySensors):
			return readingsError(c, http.StatusBadRequest, "Empty list of sensors are not allowed.")
		case errors.Is(err, request.ErrUnknownSensor):
			return readingsError(c, http.StatusBadRequest, "Unknown sensor name.")
		default:
			return readingsError(c, http.StatusInternalServerError, "Internal server error.")
		}
	}
	if query.End.Before(query.Start) || query.End.Sub(query.Start) > 24*time.Hour {
		return readingsError(c, http.StatusBadRequest, "Time range must be between 0 and 24 hours.")
	}
	result, err := s.readingsService.GetReadings(c.Request().Context(), query)
	if errors.Is(err, service.ErrNoReadings) {
		return readingsError(c, http.StatusNotFound, fmt.Sprintf("No data was found for room: %s. The room may not exist or may have no readings for the requested sensors and time range.", query.Room))
	}
	if err != nil {
		return readingsError(c, http.StatusInternalServerError, "Internal server error.")
	}
	readings, err := response.MapReadings(query, result)
	if err != nil {
		return readingsError(c, http.StatusInternalServerError, "Internal server error.")
	}
	return c.JSON(http.StatusOK, readings)
}

func readingsError(c *echo.Context, status int, message string) error {
	return c.JSON(status, errorResponse{Status: status, Message: message, Data: map[string]any{}})
}
