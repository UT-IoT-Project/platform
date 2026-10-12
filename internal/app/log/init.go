package log

import (
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/jouleion/busyness-sensing/backend/internal/app/config"
)

var Logger zerolog.Logger

func Init() {
	zerolog.TimeFieldFormat = time.RFC3339
	Logger = zerolog.New(os.Stdout).With().Timestamp().Caller().Logger().Level(config.C.Logger.Level)
}
