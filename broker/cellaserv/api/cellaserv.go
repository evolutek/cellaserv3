package api

type ClientJSON struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type ServiceJSON struct {
	Client         string `json:"client"`
	Name           string `json:"name"`
	Identification string `json:"identification"`
}

// Cellaserv service

type NameClientRequest struct {
	Name string
}

type RegisterServiceRequest struct {
	Name           string
	Identification string
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

type EventInfoJSON struct {
	Event       string   `json:"event"`
	Subscribers []string `json:"subscribers"`
}

type ListEventsResponse []EventInfoJSON
