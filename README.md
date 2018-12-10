# cellaserv3

[![Build Status](https://travis-ci.com/evolutek/cellaserv3.svg?branch=master)](https://travis-ci.com/evolutek/cellaserv3)
[![GoDoc](https://godoc.org/bitbucket.org/evolutek/cellaserv3?status.svg)](https://godoc.org/bitbucket.org/evolutek/cellaserv3)
[![Go Report Card](https://goreportcard.com/badge/bitbucket.org/evolutek/cellaserv3)](https://goreportcard.com/report/bitbucket.org/evolutek/cellaserv3)

This repository contains:

- `cellaserv`, the RPC broker
- `cellaservctl`, the command line tool to for cellaserv
- `client` the go client library for cellaserv

## Usage

cellaserv:

```
go get bitbucket.org/evolutek/cellaserv3/cmd/cellaserv && cellaserv
```

cellaservctl:

```
go get bitbucket.org/evolutek/cellaserv3/cmd/cellaservctl && cellaservctl
```

### Configuration

See `cellaserv --help`.

## Concepts and features

Cellaserv supports both the request/reply and subscribe/publish communication
primitives.

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

Any connection can send a subscribe message and receive publish messages whose
event string matches the subscribed pattern. The subscribe pattern syntax is
https://golang.org/pkg/path/filepath/#Match.

## Advanced features and concepts

### HTTP interface

By default, the HTTP interface is started on the `:4280` port. It displays the
current status of cellaserv.

### Connections

TODO: document


### Introspection with the built-in cellaserv service

### Spying on services

Any connection can ask to be sent a carbon copy of requests and responses
to/from a service by sending a `spy` request to the `cellaserv` service.

The client library has to support receiving messages that are not addressed to
the services it manages.

The prototype of the `spy` request is the following:

```
cellaserv.spy(serviceName string, serviceIdentification string)
```

### Logging

TODO: document

## TODO

* P0 cellaserv: clean the web directory, only keep the REST interface and minimalistic html interface
* P1 cellaservctl: add logs command
* P1 client: prometheus monitoring in the go client
* P1 use errors.Wrapf
* P1 store everything related to a connection to an internal conn struct, simplifies locking
* P1 decouple services and spies
* P2 fix arch linux package
* P2 client: add config variables
* P2 client: add service dependencies
* P2 common: add back `CS_DEBUG`
* P2 use structured logging
