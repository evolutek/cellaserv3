# cellaserv3

![Test Status](https://github.com/evolutek/cellaserv3/workflows/Test/badge.svg)
![GoDoc](https://godoc.org/github.com/evolutek/cellaserv3?status.svg)
![Go Report Card](https://goreportcard.com/badge/github.com/evolutek/cellaserv3)

This repository contains:

- `cellaserv`, the RPC broker, with it's web interface, REST api and debugging
  tools.
- `cellaservctl`, the command line tool to for cellaserv
- `client` the go client library for cellaserv

## Usage

cellaserv:

```
go get github.com/evolutek/cellaserv3/cmd/cellaserv && cellaserv
```

cellaservctl:

```
go get github.com/evolutek/cellaserv3/cmd/cellaservctl && cellaservctl
```

## Testing

Run:

```
go test ./...
```

### Configuration

See `cellaserv --help` and `cellaservctl --help`.

## Concepts and features

Cellaserv supports both the request/reply and subscribe/publish communication
primitives.

### Clients

* A cellaserv client is created for each connection.
* A client has a unique and stable identifier, and a name.
* By default, the name of the client is it's id, but the client can change it
  using the cellaserv internal service.

### Services

* Any client can registered any number of service.
* A service is registered by sending an `<name, identifcation>` pair to cellaserv.
* A service is referenced by two strings: its name and identification. In
  object oriented design, the name of the service is the class and the
  identification is an instance.
* If a client register a service that is already present in cellaserv, the old
  service is replaced by the new.
* The singleton instance is implemented with `identification==""`.
* No method are mandatory, also some are commonly implemented by clients:

  * `ping()` to check that the service is alive
  * `status() string` to get a one-line status of the service
  * `doc() string` to get the full documentation of a service
  * `quit()` to quit the service

### Requests

* Any cellaserv client can send a request.
* Requests are directed to a single `<service, identification, method>` trio.
* Requests can have data attached, which is stored as a binary blob, but most
  client expect a JSON serialized message. The Request data is usually the
  arguments of the method.
* Replies can also have data attached, as a binary blob too and likewise,
  usually used to store a serialized JSON message. They also have a dedicated
  error field which can be set by the service, in case the request data was
  invalid, for example.
* Replies should be sent in a short (<5 seconds by default) amount of time,
  otherwise cellaserv will send a timeout reply error on behalf of the service.

### Subscribes

* Any client can send a subscribe message and receive publish messages whose
event string matches the subscribed pattern. The subscribe pattern syntax is
https://golang.org/pkg/path/filepath/#Match.

## Advanced features and concepts

### HTTP interface

By default, the HTTP interface is started on the `:4280` port. It displays the
current status of cellaserv.

### Cellaserv bult-in service

TODO

### Spying on services

Any client can ask to be sent a carbon copy of requests and responses
to/from a service by sending a `spy` request to the `cellaserv` service.

The client library has to support receiving messages that are not addressed to
the services it manages.

The prototype of the `spy` request is the following:

```
cellaserv.spy(serviceName string, serviceIdentification string)
```
