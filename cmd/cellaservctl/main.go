package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/client"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	request     = kingpin.Command("request", "Makes a request to a service.").Alias("r")
	requestPath = request.Arg("path", "request path: service.method, service/id.method").Required().String()
	requestArgs = request.Arg("args", "args for the method").StringMap()

	publish      = kingpin.Command("publish", "Sends a publish event.").Alias("p")
	publishEvent = publish.Arg("event", "The event name to publish.").Required().String()
	publishArgs  = publish.Arg("args", "The content of event to publish.").StringMap()

	subscribe        = kingpin.Command("subscribe", "Listens for an event.").Alias("s")
	subscribeEvent   = subscribe.Arg("event", "Event name pattern to subscribe to.").Required().String()
	subscribeMonitor = subscribe.Flag("monitor", "Instead of exiting after received a single event, execute indefinitely.").Short('m').Bool()

	listServices = kingpin.Command("list-services", "Lists services currently registered.").Alias("ls")

	listConnections = kingpin.Command("list-connections", "Lists connections currently established.").Alias("lc")
)

func main() {
	// Connect to cellaserv
	client := client.NewConnection(":4200")

	switch kingpin.Parse() {
	case "request":
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
	case "publish":
		client.Publish(*publishEvent, *publishArgs)
	case "subscribe":
		eventPattern, err := regexp.Compile(*subscribeEvent)
		kingpin.FatalIfError(err, "Invalid event name pattern: %s", *subscribeEvent)
		err = client.Subscribe(eventPattern, func(eventName string, eventBytes []byte) {
			// Decode
			var eventData interface{}
			json.Unmarshal(eventBytes, &eventData)
			// Display
			fmt.Printf("%s: %v\n", eventName, eventData)

			// Should exit?
			if !*subscribeMonitor {
				client.Close()
			}
		})
		kingpin.FatalIfError(err, "Could no subscribe")
		<-client.Quit()
	case "list-services":
		// Create stub
		stub := client.NewServiceStub("cellaserv", "")
		// Make request
		respBytes := stub.Request("list-services", nil)
		// Decode response
		var services []broker.ServiceJSON
		json.Unmarshal(respBytes, &services)
		// Display services
		for _, service := range services {
			fmt.Print(service.Name)
			if service.Identification != "" {
				fmt.Printf("[%s]", service.Identification)
			}
			fmt.Print("\n")
		}
	case "list-connections":
		// Create stub
		stub := client.NewServiceStub("cellaserv", "")
		// Make request
		respBytes := stub.Request("list-connections", nil)
		// Decode response
		var connections []broker.ConnNameJSON
		json.Unmarshal(respBytes, &connections)
		// Display connections
		for _, connection := range connections {
			fmt.Printf("%s %s\n", connection.Addr, connection.Name)
		}
	}
}
