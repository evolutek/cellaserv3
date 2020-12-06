package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/evolutek/cellaserv3/client"
)

const (
	WORKERS            = 50
	REQUESTS_BY_WORKER = 1000
)

func bench(wg *sync.WaitGroup) {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})
	// Create date service stub
	date := client.NewServiceStub(conn, "date", "")

	for i := 0; i < REQUESTS_BY_WORKER; i++ {
		// Request date.time()
		respBytes, err := date.Request("time", nil)
		if err != nil {
			log.Printf("date.time() query failed: %s", err)
			return
		}
		// Process response
		var resp time.Time
		err = json.Unmarshal(respBytes, &resp)
		if err != nil {
			log.Printf("Could not unmarshal response: %s", err)
			return
		}
	}
	wg.Done()
}

func main() {
	fmt.Printf("Starting %d workers doing %d requests\n", WORKERS, REQUESTS_BY_WORKER)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < WORKERS; i++ {
		wg.Add(1)
		go bench(&wg)
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Println(elapsed)
}
