package common

import (
	"os"
	"sync"

	logging "github.com/op/go-logging"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var appLogLevel = logging.ERROR
var logBackendInit sync.Once
var loggingMtx sync.Mutex

type loggerSettings struct {
	level string
}

func (s *loggerSettings) apply(ctx *kingpin.ParseContext) error {
	var err error
	appLogLevel, err = logging.LogLevel(s.level)
	return err
}

// AddFlags adds the flags used by this package to the Kingpin application.
// To use the default Kingpin application, call AddFlags(kingpin.CommandLine)
func AddFlags(a *kingpin.Application) {
	s := loggerSettings{}
	a.Flag("log-level", "Only log messages with the given severity or above. Valid levels: [debug, info, warning, error, critical]").
		Default("debug").
		StringVar(&s.level)
	a.Action(s.apply)
}

func NewLogger(module string) *logging.Logger {
	logBackendInit.Do(func() {
		format := logging.MustStringFormatter("%{level:-7s} %{time:Jan _2 15:04:05.000} %{message}")
		logging.SetFormatter(format)

		logBackend := logging.NewLogBackend(os.Stderr, "", 0)
		logBackend.Color = true
		logging.SetBackend(logBackend)
	})

	logger := logging.MustGetLogger(module)
	loggingMtx.Lock()
	logging.SetLevel(appLogLevel, module)
	loggingMtx.Unlock()

	return logger
}
