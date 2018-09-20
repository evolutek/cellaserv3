package common

import "testing"

func TestLog(t *testing.T) {
	LogSetup()
	LogEvent("testevent", "testmsg")
}
