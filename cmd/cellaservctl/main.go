package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/evolutek/cellaserv3/client"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	request     = kingpin.Command("request", "Make a request to a service.")
	requestPath = request.Arg("path", "request path: service.method, service/id.method or service[id].method").Required().String()
	requestArgs = request.Arg("args", "args for the method").String()
)

func main() {
	switch kingpin.Parse() {
	case "request":
		client := client.NewConnection(":4200")
		// Parse service and method
		requestPathSlice := strings.Split(*requestPath, ".")
		requestService := requestPathSlice[0]
		requestMethod := requestPathSlice[1]
		service := client.NewServiceStub(requestService, "")

		// Serialize request args
		var requestPayload interface{}
		json.Unmarshal([]byte(*requestArgs), &requestPayload)

		// Make request
		respBytes := service.Request(requestMethod, requestPayload)

		// Display response
		var requestResponse interface{}
		json.Unmarshal(respBytes, &requestResponse)
		fmt.Printf("%#v\n", requestResponse)
	}
}
