package http

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
	"github.com/jouleion/busyness-sensing/backend/internal/core/port"
)

type Server struct {
	e               *echo.Echo
	readingsService port.ReadingsService
}

func NewServer(readingsService port.ReadingsService) *Server {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	return (&Server{e: e, readingsService: readingsService}).registerRoutes()
}

func (s *Server) registerRoutes() *Server {
	s.e.GET("/api/v1/readings/:building/:roomNumber", s.getReadings)
	return s
}

func (s *Server) Start() error {
	address := fmt.Sprintf("%s:%s", config.C.HTTP.Address, config.C.HTTP.Port)
	return s.e.Start(address)
}
