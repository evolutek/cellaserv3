package common

import (
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// Logger is the interface for loggers used in the Prometheus components.
type Logger interface {
	Debug(...interface{})
	Debugln(...interface{})
	Debugf(string, ...interface{})

	Info(...interface{})
	Infoln(...interface{})
	Infof(string, ...interface{})

	Warn(...interface{})
	Warnln(...interface{})
	Warnf(string, ...interface{})

	Error(...interface{})
	Errorln(...interface{})
	Errorf(string, ...interface{})

	Fatal(...interface{})
	Fatalln(...interface{})
	Fatalf(string, ...interface{})
}

type loggerSettings struct {
	level string
	trace bool
}

func (s *loggerSettings) apply(ctx *kingpin.ParseContext) error {
	lvl, err := logrus.ParseLevel(s.level)
	if err != nil {
		return err
	}
	log.SetReportCaller(s.trace)
	log.SetLevel(lvl)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return nil
}

// AddFlags adds the flags used by this package to the Kingpin application.
// To use the default Kingpin application, call AddFlags(kingpin.CommandLine)
func AddFlags(a *kingpin.Application) {
	s := loggerSettings{}
	a.Flag("log-level", "Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, fatal]").
		Default("debug").
		StringVar(&s.level)
	a.Flag("log-trace", "Whether to include function and file information in the log.").
		BoolVar(&s.trace)
	a.Action(s.apply)
}

func NewLogger(module string) Logger {
	contextLogger := log.WithField("module", module)
	return contextLogger
}
