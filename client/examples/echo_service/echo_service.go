package main

import (
	"encoding/json"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/client"
)

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})

	// Prepare service for registration
	echo := conn.NewService("echo", "")
	// Handle "time" request
	echo.HandleRequestFunc("echo", func(req *cellaserv.Request) (interface{}, error) {
		// Parse json from request
		var reqObj interface{}
		err := json.Unmarshal(req.GetData(), &reqObj)
		return reqObj, err
	})
	// Handle "killall" event
	echo.HandleEventFunc("killall", func(_ *cellaserv.Publish) {
		conn.Close()
	})

	// Register the service on cellaserv
	conn.RegisterService(echo)

	<-conn.Quit()
}
