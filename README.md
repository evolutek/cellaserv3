# cellaserv3

[![Build Status](https://travis-ci.com/evolutek/cellaserv3.svg?branch=master)](https://travis-ci.com/evolutek/cellaserv3)
[![GoDoc](https://godoc.org/github.com/evolutek/cellaserv3?status.svg)](https://godoc.org/github.com/evolutek/cellaserv3)
[![Go Report Card](https://goreportcard.com/badge/github.com/evolutek/cellaserv3)](https://goreportcard.com/report/github.com/evolutek/cellaserv3)

## TODO

* prometheus monitoring
* http UI
* cellaservctl
* add support for remote cellaserv: `CS_ADDR`
* replace golang logging with op/go-logging
* evaluate interest of logs and sessions, maybe setup logs access over HTTP?

## Main features

Cellaserv supports both the request/reply and subscribe/publish communication methods.

## Advanced features

These features are mainly required for new tools. They use existing
infrastructure to be implemented.

### Spying on services

Any connection can ask to get a carbon copy of requests and responses to/from a
service by sending a `spy` request to the `cellaserv` service.

The prototype of the request is the following:

```
cellaserv.spy(serviceName string, serviceIdentification string)
```
