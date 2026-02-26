package log

import (
	"os"

	"github.com/rs/zerolog"
)

var Logger zerolog.Logger

func init() {
	// Always use ConsoleWriter to stderr for logs
	output := zerolog.ConsoleWriter{Out: os.Stderr}
	Logger = zerolog.New(output).With().Timestamp().Logger()
}
