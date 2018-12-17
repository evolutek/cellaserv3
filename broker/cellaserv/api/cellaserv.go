package api

type ClientJSON struct {
	Id   string
	Name string
}

type ServiceJSON struct {
	Addr           string
	Name           string
	Identification string
}

// Cellaserv service

type NameClientRequest struct {
	Name string
}

type SpyRequest struct {
	ServiceName           string
	ServiceIdentification string
	ClientId              string
}

type GetLogsRequest struct {
	Pattern string
}

type GetLogsResponse map[string]string

// TODO(halfr): make that a *Response
type EventsJSON map[string][]string
