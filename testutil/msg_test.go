package testutil

import (
	"testing"
)

func TestMakeMessageRegister(t *testing.T) {
	const serviceName = "testName"
	const serviceIdent = "testIdent"
	MakeMessageRegister(t, serviceName, serviceIdent)
}

func TestMakeMessageRequest(t *testing.T) {
	const serviceName = "testName"
	const serviceIdent = "testIdent"
	const method = "method"
	var payload []byte
	MakeMessageRequest(t, serviceName, serviceIdent, method, payload)
}

func TestMakeMessageReply(t *testing.T) {
	MakeMessageReply(t, 42, nil)
	payload := []byte{42, 42}
	MakeMessageReply(t, 42, payload)
}
