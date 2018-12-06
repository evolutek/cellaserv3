package common

import (
	"os"

	logging "github.com/op/go-logging"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var baseLogLevel = logging.DEBUG

type loggerSettings struct {
	level string
}

func (s *loggerSettings) apply(ctx *kingpin.ParseContext) error {
	baseLogLevel = logging.GetLevel(s.level)
	return nil
}

// AddFlags adds the flags used by this package to the Kingpin application.
// To use the default Kingpin application, call AddFlags(kingpin.CommandLine)
func AddFlags(a *kingpin.Application) {
	s := loggerSettings{}
	a.Flag("log.level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]").
		Default("debug").
		StringVar(&s.level)
	a.Action(s.apply)
}

func NewLogger(module string) *logging.Logger {
	format := logging.MustStringFormatter("%{level:-7s} %{time:Jan _2 15:04:05.000} %{message}")
	logging.SetFormatter(format)

	logBackend := logging.NewLogBackend(os.Stderr, "", 0)
	logBackend.Color = true
	logging.SetBackend(logBackend)

	logger := logging.MustGetLogger(module)
	logging.SetLevel(baseLogLevel, module)

	return logger
}
