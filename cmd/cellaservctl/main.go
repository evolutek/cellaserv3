// Command line interface for cellaserv.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cellaserv "github.com/evolutek/cellaserv3-protobuf"
	"github.com/evolutek/cellaserv3/broker/cellaserv/api"
	"github.com/evolutek/cellaserv3/client"
	"github.com/evolutek/cellaserv3/common"
	"github.com/pkg/errors"

	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

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
	a := kingpin.New(filepath.Base(os.Args[0]), "Control the cellaserv broker")
	a.Version(common.GetVersion())
	a.HelpFlag.Short('h')

	request := a.Command("request", "Makes a request to a service. Alias: r").Alias("r")
	requestPath := request.Arg("path", "Request path. Example service.method or service/id.method.").Required().String()
	requestArgs := request.Arg("args", "Key=value arguments of the method. Example: x=42 y=43").StringMap()
	requestRaw := request.Flag("raw", "Do not decode response as JSON").Bool()

	publish := a.Command("publish", "Sends a publish event. Alias: p").Alias("p")
	publishEvent := publish.Arg("event", "Event name to publish.").Required().String()
	publishArgs := publish.Arg("args", "Key=value content of event to publish. Example: x=42 y=43").StringMap()
	publishRaw := publish.Flag("raw", "Raw bytes to send as publish data").String()

	subscribe := a.Command("subscribe", "Listens for an event. Alias: s").Alias("s")
	subscribeEventPattern := subscribe.Arg("event", "Event name pattern to subscribe to.").Required().String()
	subscribeMonitor := subscribe.Flag("monitor", "Instead of exiting after received a single event, wait indefinitely.").Short('m').Bool()

	log := a.Command("log", "Get logs. Alias: l").Alias("l")
	logPattern := log.Arg("pattern", "Log name pattern. Example: 'cellaserv.new-client'").Required().String()
	logFolow := log.Flag("follow", "Instead of exiting after received logs, wait for new.").Short('f').Bool()

	spy := a.Command("spy", "Listens to all requests and responses of a service.")
	spyPath := spy.Arg("path", "Spy path. Example service or service/id").Required().String()

	a.Command("list-services", "Lists services currently registered. Alias: ls").Alias("ls")

	a.Command("list-clients", "Lists cellaserv's clients. Alias: lc").Alias("lc")

	common.AddFlags(a)

	command, err := a.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, errors.Wrapf(err, "Could not parse command line arguments"))
		a.Usage(os.Args[1:])
		os.Exit(2)
	}

	// Connect to cellaserv
	conn := client.NewClient(client.ClientOpts{})

	switch command {
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

		if !*requestRaw {
			// Display response
			var requestResponse interface{}
			json.Unmarshal(respBytes, &requestResponse)
			pretty, _ := json.MarshalIndent(requestResponse, "", "  ")
			fmt.Println(string(pretty))
		} else {
			fmt.Printf("%s\n", respBytes)
		}
	case "publish":
		if *publishRaw != "" {
			conn.PublishRaw(*publishEvent, []byte(*publishRaw))
		} else {
			conn.Publish(*publishEvent, *publishArgs)
		}
	case "subscribe":
		err := conn.Subscribe(*subscribeEventPattern,
			func(eventName string, eventBytes []byte) {
				fmt.Printf("%s: %s\n", eventName, string(eventBytes))

				// Should exit?
				if !*subscribeMonitor {
					conn.Close()
				}
			})
		kingpin.FatalIfError(err, "Could no subscribe")
		<-conn.Quit()
	case "log":
		// Log with follow is just a special case of "subscribe"
		if *logFolow {
			err := conn.Subscribe("log."+*logPattern,
				func(eventName string, eventBytes []byte) {
					fmt.Printf("%s: %s\n", eventName, string(eventBytes))
				})
			kingpin.FatalIfError(err, "Could no subscribe")
			<-conn.Quit()
		} else {
			// Create service stub
			stub := client.NewServiceStub(conn, "cellaserv", "")
			// Make request
			respBytes, err := stub.Request("get_logs", &api.GetLogsRequest{*logPattern})
			kingpin.FatalIfError(err, "Request failed")
			var getLogsResponse api.GetLogsResponse
			err = json.Unmarshal(respBytes, &getLogsResponse)
			kingpin.FatalIfError(err, "Unmarshal failed")
			for event, logs := range getLogsResponse {
				fmt.Println(">>> ", event)
				fmt.Println(logs)
			}
		}
	case "spy":
		// Parse args
		service, identification := common.ParseServicePath(*spyPath)
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
		respBytes, err := stub.Request("list_services", nil)
		kingpin.FatalIfError(err, "Request failed")
		// Decode response
		var services []api.ServiceJSON
		err = json.Unmarshal(respBytes, &services)
		kingpin.FatalIfError(err, "Unmarshal of reply data failed")
		// Display services
		for _, service := range services {
			fmt.Print(service.Name)
			if service.Identification != "" {
				fmt.Printf("/%s", service.Identification)
			}
			fmt.Print("\n")
		}
	case "list-clients":
		// Create service stub
		stub := client.NewServiceStub(conn, "cellaserv", "")
		// Make request
		respBytes, err := stub.Request("list_clients", nil)
		kingpin.FatalIfError(err, "Request failed")
		// Decode response
		var connections []api.ClientJSON
		err = json.Unmarshal(respBytes, &connections)
		kingpin.FatalIfError(err, "Unmarshal of reply data failed")
		// Display connections
		for _, connection := range connections {
			fmt.Printf("%s %s\n", connection.Id, connection.Name)
		}
	}
}
