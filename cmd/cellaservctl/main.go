package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evolutek/cellaserv3/client"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	request     = kingpin.Command("request", "Make a request to a service.").Alias("r")
	requestPath = request.Arg("path", "request path: service.method, service/id.method").Required().String()
	requestArgs = request.Arg("args", "args for the method").StringMap()
)

func main() {
	switch kingpin.Parse() {
	case "request":
		// Connect to cellaserv
		client := client.NewConnection(":4200")

		// Parse service and method
		requestPathSlice := strings.Split(*requestPath, ".")
		requestService := requestPathSlice[0]
		requestMethod := requestPathSlice[1]

		// Check for identification
		var requestServiceIdentification string
		requestMaybeIdentificationSlice := strings.Split(requestService, "/")

		if len(requestMaybeIdentificationSlice) == 2 {
			// The user wrote service/id.method
			requestService = requestMaybeIdentificationSlice[0]
			requestServiceIdentification = requestMaybeIdentificationSlice[1]
		}

		// Create service stub
		service := client.NewServiceStub(requestService, requestServiceIdentification)

		// Make request
		respBytes := service.Request(requestMethod, requestArgs)

		// Display response
		var requestResponse interface{}
		json.Unmarshal(respBytes, &requestResponse)
		fmt.Printf("%#v\n", requestResponse)
	}
}
