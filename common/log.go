// TODO: refactor to use oklog
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
	logRootDirectory = flag.String("log-root", ".", "root directory of logs")
	logSubDir        string
	logLevelFlag     = flag.String("log-level", "0", "logger verbosity (0 = WARNING, 1 = INFO, 2 = DEBUG)")
	logToFile        = flag.String("log-file", "", "log to custom file instead of stderr")

	// Map of the logger associated with a service
	servicesLogs map[string]*golog.Logger
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

func LogSetup() {
	if log != nil {
		log.Error("Logs are already initialized.")
		return
	}

	format := logging.MustStringFormatter("%{level:-7s} %{time:Jan _2 15:04:05.000} %{message}")

	var logBackend *logging.LogBackend
	if *logToFile != "" {
		// Use has specified a log file to use
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
	// Set default log subDirectory to now
	logRotateTimeNow()
}

// logRotateName set the new log subdirectory to name
func logRotateName(name string) {
	log.Debug("[Log] Rotating to \"%s\"", name)
	logSubDir = name
	logFullDir := path.Join(*logRootDirectory, logSubDir)
	err := os.MkdirAll(logFullDir, 0755)
	if err != nil {
		log.Error("[Log] Could not create log directories, %s: %s", logFullDir, err)
	}
	// XXX: close old log files?
	servicesLogs = make(map[string]*golog.Logger)
}

// logRotateTimeNow switch the current log subdirectory to current time
func logRotateTimeNow() {
	now := time.Now()
	newSubDir := now.Format(time.Stamp)
	logRotateName(newSubDir)
}

func logSetupFile(what string) (l *golog.Logger) {
	l, ok := servicesLogs[what]
	if !ok {
		logFilename := path.Join(*logRootDirectory, logSubDir, what+".log")
		logFd, err := os.OpenFile(logFilename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
		if err != nil {
			log.Error("[Log] Could not create log file: %s", logFilename)
			return
		}
		l = golog.New(logFd, what, golog.LstdFlags)
		l.SetPrefix("")
		servicesLogs[what] = l
	}
	return
}

func LogEvent(event string, what string) {
	logger, ok := servicesLogs[event]
	if !ok {
		logger = logSetupFile(event)
		if logger == nil {
			return
		}
	}
	logger.Println(what)
}

func GetLog() *logging.Logger {
	return log
}
