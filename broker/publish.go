package broker

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

func (b *Broker) handlePublish(c *client, msgBytes []byte, pub *cellaserv.Publish) {
	b.logger.Infof("[Publish] %s publishes %s", c, pub.Event)
	b.doPublish(msgBytes, pub)
}

func (b *Broker) doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	// Holds subscribers for this publish
	var subs []*client

	// Handle log publishes
	if b.Options.PublishLoggingEnabled && strings.HasPrefix(pub.Event, "log.") {
		loggingEvent := pub.Event[len("log."):]
		data := string(pub.Data) // expect data to be utf8
		b.handleLoggingPublish(loggingEvent, data)
	}

	// Handle glob susbscribers
	b.subscriberMapMtx.RLock()
	for pattern, clients := range b.subscriberMatchMap {
		matched, _ := filepath.Match(pattern, pub.Event)
		if matched {
			subs = append(subs, clients...)
		}
	}
	b.subscriberMapMtx.RUnlock()

	// Add exact matches
	subs = append(subs, b.subscriberMap[pub.Event]...)

	for _, c := range subs {
		b.logger.Debugf("[Publish] %s → %s", pub.Event, c)
		b.sendRawMessage(c.conn, msgBytes)
	}
}

func (b *Broker) rotatePublishLoggers() error {
	b.publishLoggingSession = time.Now().Format(time.RFC3339)
	b.publishLoggingRoot = path.Join(b.Options.VarRoot, "logs", b.publishLoggingSession)
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
