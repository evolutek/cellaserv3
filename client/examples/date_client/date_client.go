package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"
)

var log = common.GetLog()

func main() {
	// Connect to cellaserv
	conn := client.NewConnection(":4200")
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
