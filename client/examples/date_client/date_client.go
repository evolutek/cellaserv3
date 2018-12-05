package main

import (
	"encoding/json"
	"fmt"
	"time"

	"bitbucket.org/evolutek/cellaserv3/client"
	"bitbucket.org/evolutek/cellaserv3/common"
)

var log = common.GetLog()

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})
	// Create date service stub
	date := client.NewServiceStub(conn, "date", "")
	// Request date.time()
	respBytes, err := date.Request("time", nil)
	if err != nil {
		log.Error("date.time() query failed: %s", err)
		return
	}
	var resp time.Time
	// Process response
	json.Unmarshal(respBytes, &resp)
	fmt.Println(resp)
}
