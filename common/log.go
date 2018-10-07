package common

import (
	"flag"
	golog "log"
	"os"
	"path"
	"time"

	logging "gopkg.in/op/go-logging.v1"
)

var (
	// log is the main cellaserv logger, use it everywhere you want!
	log *logging.Logger

	// Command line flags
	logRootDirectory = flag.String("log-dir", "", "root directory of logs, empty means do not write logs to files")
	logSubDir        string
	logLevelFlag     = flag.String("log-level", "2", "logger verbosity (0 = WARNING, 1 = INFO, 2 = DEBUG)")
	logToFile        = flag.String("log-file", "", "log to file instead of stderr")

	// Maps events to their dedicated loggers
	loggers map[string]*golog.Logger
)

func getLogLevel() logging.Level {
	switch *logLevelFlag {
	case "0":
		return logging.WARNING
	case "1":
		return logging.INFO
	case "2":
		return logging.DEBUG
	}

	// Fallback
	log.Warning("[Config] Unknown debug value: %s", *logLevelFlag)
	return logging.WARNING
}

// LogSetup configures the loggging subsystem. Returns the name of the logging directory
func LogSetup() {
	if log != nil {
		log.Warning("Logs are already initialized.")
		return
	}

	format := logging.MustStringFormatter("%{level:-7s} %{time:Jan _2 15:04:05.000} %{message}")

	var logBackend *logging.LogBackend
	if *logToFile != "" {
		// User has specified a log file to use
		logFile, err := os.OpenFile(*logToFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			golog.Println(err)
			golog.Println("Falling back on log on stderr")
			logBackend = logging.NewLogBackend(os.Stderr, "", 0)
		} else {
			logBackend = logging.NewLogBackend(logFile, "", 0)
		}
	} else {
		// Log on stderr
		logBackend = logging.NewLogBackend(os.Stderr, "", 0)
	}
	logBackend.Color = true
	logging.SetBackend(logBackend)

	logging.SetFormatter(format)
	log = logging.MustGetLogger("cellaserv")

	logging.SetLevel(getLogLevel(), "cellaserv")
	// Set default log directory to now
	logRotateTimeNow()
}

// logRotate changes the log directory
func logRotate(name string) {
	if *logRootDirectory != "" {
		log.Debug("[Log] Writing new logs to: %s", name)
		logSubDir = name
		logFullDir := path.Join(*logRootDirectory, logSubDir)
		err := os.MkdirAll(logFullDir, 0755)
		if err != nil {
			log.Error("[Log] Could not create log directory, %s: %s", logFullDir, err)
		}
	}
	// XXX: close old log files?
	loggers = make(map[string]*golog.Logger)
}

// logRotateTimeNow switches the current log directory to a new one, named
// after the current time and date.
func logRotateTimeNow() {
	now := time.Now()
	newSubDir := now.Format(time.Stamp)
	logRotate(newSubDir)
}

// loggerSetup setup and returns a new logger
func loggerSetup(logName string) *golog.Logger {
	logger, found := loggers[logName]
	if !found {
		if *logRootDirectory != "" {
			logFilename := path.Join(*logRootDirectory, logSubDir, logName+".log")
			logFd, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
			if err != nil {
				log.Error("[Log] Could not create log file: %s", logFilename)
				return nil
			}
			logger = golog.New(logFd, logName, golog.LstdFlags)
			logger.SetPrefix("")
		} else {
			logger = golog.New(os.Stderr, logName, golog.LstdFlags)
		}
		loggers[logName] = logger
	}
	return logger
}

// LogEvent writes a log entry to one of the event logs
func LogEvent(event string, msg string) {
	logger, found := loggers[event]
	if !found {
		logger = loggerSetup(event)
		if logger == nil {
			return
		}
	}
	logger.Println(msg)
}

func GetLog() *logging.Logger {
	if log == nil {
		LogSetup()
	}
	return log
}
