package http

import (
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
)

type Server struct {
	e *echo.Echo
}

func NewServer() *Server {
	e := echo.New()
	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	return &Server{e: e}
}

func (s *Server) Start() error {
	address := fmt.Sprintf("%s:%s", config.C.HTTP.Address, config.C.HTTP.Port)
	return s.e.Start(address)
}
