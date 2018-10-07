package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
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

	spy     = kingpin.Command("spy", "Listens to all requests and responses to a service.")
	spyPath = spy.Arg("path", "spy path: service or service/id").Required().String()

	listServices = kingpin.Command("list-services", "Lists services currently registered.").Alias("ls")

	listConnections = kingpin.Command("list-connections", "Lists connections currently established.").Alias("lc")
)

func parseServicePath(path string) (string, string) {
	pathSlice := strings.Split(path, ".")
	service := pathSlice[0]

	// Default identification
	var identification string

	// Extract identification, if present
	identificationSlice := strings.Split(service, "/")
	if len(identificationSlice) == 2 {
		service = identificationSlice[0]
		identification = identificationSlice[1]
	}
	return service, identification
}

func requestToString(req *cellaserv.Request) string {
	var reqData interface{}
	_ = json.Unmarshal(req.GetData(), &reqData)
	return fmt.Sprintf("%s/%s.%s(%v)", req.GetServiceName(), req.GetServiceIdentification(), req.GetMethod(), reqData)
}

func replyToString(rep *cellaserv.Reply) string {
	var repData interface{}
	_ = json.Unmarshal(rep.GetData(), &repData)
	return fmt.Sprintf("%v", repData)
}

func main() {
	// Connect to cellaserv
	conn := client.NewConnection(":4200")

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
		service := client.NewServiceStub(conn, requestService, requestServiceIdentification)

		// Make request
		respBytes, err := service.Request(requestMethod, requestArgs)
		kingpin.FatalIfError(err, "Request failed")

		// Display response
		var requestResponse interface{}
		json.Unmarshal(respBytes, &requestResponse)
		fmt.Printf("%#v\n", requestResponse)
	case "publish":
		conn.Publish(*publishEvent, *publishArgs)
	case "subscribe":
		eventPattern, err := regexp.Compile(*subscribeEvent)
		kingpin.FatalIfError(err, "Invalid event name pattern: %s", *subscribeEvent)
		err = conn.Subscribe(eventPattern,
			func(eventName string, eventBytes []byte) {
				// Decode
				var eventData interface{}
				json.Unmarshal(eventBytes, &eventData)
				// Display
				fmt.Printf("%s: %v\n", eventName, eventData)

				// Should exit?
				if !*subscribeMonitor {
					conn.Close()
				}
			})
		kingpin.FatalIfError(err, "Could no subscribe")
		<-conn.Quit()
	case "spy":
		service, identification := parseServicePath(*spyPath)
		conn.Spy(service, identification,
			func(req *cellaserv.Request, rep *cellaserv.Reply) {
				fmt.Printf("%s: %s\n", requestToString(req),
					replyToString(rep))
			})
		<-conn.Quit()
	case "list-services":
		// Create stub
		stub := client.NewServiceStub(conn, "cellaserv", "")
		// Make request
		respBytes, err := stub.Request("list-services", nil)
		kingpin.FatalIfError(err, "Request failed")
		// Decode response
		var services []broker.ServiceJSON
		err = json.Unmarshal(respBytes, &services)
		kingpin.FatalIfError(err, "Unmarshal of reply data failed")
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
		stub := client.NewServiceStub(conn, "cellaserv", "")
		// Make request
		respBytes, err := stub.Request("list-connections", nil)
		kingpin.FatalIfError(err, "Request failed")
		// Decode response
		var connections []broker.ConnNameJSON
		err = json.Unmarshal(respBytes, &connections)
		kingpin.FatalIfError(err, "Unmarshal of reply data failed")
		// Display connections
		for _, connection := range connections {
			fmt.Printf("%s %s\n", connection.Addr, connection.Name)
		}
	}
}
