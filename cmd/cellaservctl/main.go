// Command line interface for cellaserv.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	cellaserv "bitbucket.org/evolutek/cellaserv2-protobuf"
	"github.com/evolutek/cellaserv3/broker"
	"github.com/evolutek/cellaserv3/client"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	request     = kingpin.Command("request", "Makes a request to a service. Alias: r").Alias("r")
	requestPath = request.Arg("path", "Request path. Example service.method or service/id.method.").Required().String()
	requestArgs = request.Arg("args", "Key=value arguments of the method. Example: x=42 y=43").StringMap()

	publish      = kingpin.Command("publish", "Sends a publish event. Alias: p").Alias("p")
	publishEvent = publish.Arg("event", "Event name to publish.").Required().String()
	publishArgs  = publish.Arg("args", "Key=value content of event to publish. Example: x=42 y=43").StringMap()

	subscribe             = kingpin.Command("subscribe", "Listens for an event. Alias: s").Alias("s")
	subscribeEventPattern = subscribe.Arg("event", "Event name pattern to subscribe to.").Required().String()
	subscribeMonitor      = subscribe.Flag("monitor", "Instead of exiting after received a single event, wait indefinitely.").Short('m').Bool()

	spy     = kingpin.Command("spy", "Listens to all requests and responses of a service.")
	spyPath = spy.Arg("path", "Spy path. Example service or service/id").Required().String()

	listServices = kingpin.Command("list-services", "Lists services currently registered. Alias: ls").Alias("ls")

	listConnections = kingpin.Command("list-connections", "Lists connections currently established. Alias: lc").Alias("lc")
)

// Extracts service and identification information from a request path
// Supported syntaxes: service.method or service/identification.method
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

// Returns a string representation of a cellaserv.Request object.
func requestToString(req *cellaserv.Request) string {
	var reqData interface{}
	_ = json.Unmarshal(req.GetData(), &reqData)
	return fmt.Sprintf("%s/%s.%s(%v)", req.GetServiceName(), req.GetServiceIdentification(), req.GetMethod(), reqData)
}

// Returns a string representation of a cellaserv.Reply object
func replyToString(rep *cellaserv.Reply) string {
	var repData interface{}
	_ = json.Unmarshal(rep.GetData(), &repData)
	return fmt.Sprintf("%v", repData)
}

func main() {
	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})

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
		err := conn.Subscribe(*subscribeEventPattern,
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
		// Parse args
		service, identification := parseServicePath(*spyPath)
		// Setup spy with callback
		conn.Spy(service, identification,
			func(req *cellaserv.Request, rep *cellaserv.Reply) {
				fmt.Printf("%s: %s\n", requestToString(req),
					replyToString(rep))
			})
		<-conn.Quit()
	case "list-services":
		// Create service stub
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
		// Create service stub
		stub := client.NewServiceStub(conn, "cellaserv", "")
		// Make request
		respBytes, err := stub.Request("list-connections", nil)
		kingpin.FatalIfError(err, "Request failed")
		// Decode response
		var connections []broker.ConnectionJSON
		err = json.Unmarshal(respBytes, &connections)
		kingpin.FatalIfError(err, "Unmarshal of reply data failed")
		// Display connections
		for _, connection := range connections {
			fmt.Printf("%s %s\n", connection.Addr, connection.Name)
		}
	}
}
