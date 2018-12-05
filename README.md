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
methods.

### HTTP interface

By default, the HTTP interface is started on the `:4280` port. It displays the
current status of cellaserv.

### Connections

TODO: document

### Services

A service is referenced by it's name and identification. In object oriented
design, the name of the service is the class and the identification is an
instance.

### Requests

TODO: document

### Subscribes

Any connection can send a subscribe message and receive publish messages whose
event string matches the subscribed pattern. The subscribe pattern syntax is
https://golang.org/pkg/path/filepath/#Match.

## Advanced features

On top of the basic req/rep, pub/sub features, cellaserv offers

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

* P0 clean the web directory, only keep the REST interface and minimalistic html interface
* P1 backport service logging from cellaserv2
* P1 prometheus monitoring in the go client
* P2 replace golang logging with recent op/go-logging
* P2 add back `CS_DEBUG`

* client

  * P2 add config variables
  * P2 add service dependencies
