package main

import (
	"encoding/json"
	"log"

	"bitbucket.org/evolutek/cellaserv3/client"
)

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})
	err := conn.Subscribe("date", func(eventName string, eventBytes []byte) {
		// Decode
		var eventData string
		err := json.Unmarshal(eventBytes, &eventData)
		if err != nil {
			log.Printf("Could not unmarshal: %s", err)
		}
		log.Print(eventData)
	})
	if err != nil {
		log.Printf("Could not subscribe: %s", err)
		return
	}
	<-conn.Quit()
}
