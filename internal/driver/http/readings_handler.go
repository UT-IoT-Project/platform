package http

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/jouleion/busyness-sensing/backend/internal/driver/http/request"
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
	if _, err := input.ToQuery(); err != nil {
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
	return readingsError(c, http.StatusInternalServerError, "Internal server error.")
}

func readingsError(c *echo.Context, status int, message string) error {
	return c.JSON(status, errorResponse{Status: status, Message: message, Data: map[string]any{}})
}
