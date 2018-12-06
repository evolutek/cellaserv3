package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"bitbucket.org/evolutek/cellaserv3/client"
)

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})
	// Create date service stub
	date := client.NewServiceStub(conn, "date", "")
	// Request date.time()
	respBytes, err := date.Request("time", nil)
	if err != nil {
		log.Printf("date.time() query failed: %s", err)
		return
	}
	var resp time.Time
	// Process response
	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		log.Printf("Could not unmarshal response: %s", err)
		return
	}
	fmt.Println(resp)
}
