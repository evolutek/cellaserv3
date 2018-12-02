package main

import (
	"encoding/json"

	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"
)

var log = common.GetLog()

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})
	err := conn.Subscribe("date", func(eventName string, eventBytes []byte) {
		// Decode
		var eventData string
		err := json.Unmarshal(eventBytes, &eventData)
		if err != nil {
			log.Error("Could not unmarshal: ", err)
		}
		log.Info(eventData)
	})
	if err != nil {
		log.Error("Could not subscribe", err)
		return
	}
	<-conn.Quit()
}
