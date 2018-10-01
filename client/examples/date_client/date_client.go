package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/evolutek/cellaserv3/client"
)

func main() {
	// Connect to cellaserv
	client := client.NewConnection(":4200")
	// Create date service stub
	date := client.NewServiceStub("date", "")
	// Request date.time()
	respBytes := date.Request("time", nil)
	var resp time.Time
	// Process response
	json.Unmarshal(respBytes, &resp)
	fmt.Println(resp)
}
