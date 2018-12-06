package broker

import (
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
)

func (b *Broker) handlePublish(conn net.Conn, msgBytes []byte, pub *cellaserv.Publish) {
	b.logger.Infof("[Publish] %s publishes %s", b.connDescribe(conn), pub.Event)
	b.doPublish(msgBytes, pub)
}

func (b *Broker) doPublish(msgBytes []byte, pub *cellaserv.Publish) {
	event := pub.Event

	// Logging
	b.logger.Debugf("[Publish] Publishing %s", event)

	// Handle log publishes
	if b.Options.ServiceLoggingEnabled && strings.HasPrefix(event, "log.") {
		event := pub.Event[len("log."):]
		data := string(pub.Data) // expect data to be utf8
		b.handleLoggingPublish(event, data)
	}

	// Holds subscribers for this publish
	var subs []net.Conn

	// Handle glob susbscribers
	for pattern, cons := range b.subscriberMatchMap {
		matched, _ := filepath.Match(pattern, event)
		if matched {
			subs = append(subs, cons...)
		}
	}

	// Add exact matches
	subs = append(subs, b.subscriberMap[event]...)

	for _, connSub := range subs {
		b.logger.Debugf("[Publish] Forwarding %s to %s", pub.GetEvent(), b.connDescribe(connSub))
		b.sendRawMessage(connSub, msgBytes)
	}
}

func (b *Broker) rotateServiceLogs() error {
	b.serviceLoggingSession = time.Now().Format(time.RFC3339)
	b.serviceLoggingRoot = path.Join(b.Options.VarRoot, "logs", b.serviceLoggingSession)
	b.serviceLoggingLoggers = make(map[string]*os.File)
	return os.MkdirAll(b.serviceLoggingRoot, 0777)
}

func (b *Broker) serviceLoggingSetup(event string) (*os.File, error) {
	name := path.Join(b.serviceLoggingRoot, event)
	logFile, err := os.Create(name)
	return logFile, err
}

func (b *Broker) handleLoggingPublish(event string, data string) {
	logger, ok := b.serviceLoggingLoggers[event]
	if !ok {
		var err error
		logger, err = b.serviceLoggingSetup(event)
		if err != nil {
			b.logger.Errorf("[Publish] Could not create logging file for %s: %s", event, err)
			return
		}
	}
	_, err := logger.Write([]byte(data + "\n"))
	if err != nil {
		b.logger.Errorf("[Publish] Could not write to logging file %s: %s", event, err)
	}
}
