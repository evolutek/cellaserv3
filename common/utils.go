package common

import "strings"

// ParseServicePath extracts service and identification information from a
// request path. Supported syntaxes:
// - service.method or
// - service/identification.method
func ParseServicePath(path string) (string, string) {
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
