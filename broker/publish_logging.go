package broker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bitbucket.org/evolutek/cellaserv3/broker/cellaserv/api"
)

func (b *Broker) GetLogsByPattern(pattern string) (api.GetLogsResponse, error) {
	pathPattern := path.Join(b.publishLoggingRoot, pattern)

	if !strings.HasPrefix(pathPattern, b.Options.LogsDir) {
		err := fmt.Errorf("Don't try to do directory traversal: %s", pattern)
		return nil, err
	}

	// Globbing is supported
	filenames, err := filepath.Glob(pathPattern)
	if err != nil {
		err := fmt.Errorf("Invalid log globbing: %s, %s", pattern, err)
		return nil, err
	}

	logs := make(api.GetLogsResponse)

	for _, filename := range filenames {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			err := fmt.Errorf("Could not open log: %s: %s", filename, err)
			return nil, err
		}
		logs[path.Base(filename)] = string(data)
	}

	return logs, nil
}

func (b *Broker) rotatePublishLoggers() error {
	b.publishLoggingSession = time.Now().Format(time.RFC3339)
	b.publishLoggingRoot = path.Join(b.Options.LogsDir, b.publishLoggingSession)
	b.publishLoggingLoggers = sync.Map{} // reset
	return os.MkdirAll(b.publishLoggingRoot, 0777)
}

func (b *Broker) publishLoggingSetup(event string) (*os.File, error) {
	name := path.Join(b.publishLoggingRoot, event)
	logFile, err := os.Create(name)
	return logFile, err
}

func (b *Broker) handleLoggingPublish(event string, data string) {
	var logger *os.File
	loggerIface, ok := b.publishLoggingLoggers.Load(event)
	if ok {
		logger = loggerIface.(*os.File)
	} else {
		var err error
		logger, err = b.publishLoggingSetup(event)
		if err != nil {
			b.logger.Errorf("[Publish] Could not create logging file for %s: %s", event, err)
			return
		}
		b.publishLoggingLoggers.Store(event, logger)
	}

	if strings.ContainsRune(data, '\n') {
		b.logger.Warningf("[Publish] Logging for %s contains '\\n': %s", event, data)
	}

	_, err := logger.Write([]byte(data + "\n"))
	if err != nil {
		b.logger.Errorf("[Publish] Could not write to logging file %s: %s", event, err)
	}
}
