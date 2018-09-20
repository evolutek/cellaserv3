package testutil

import (
	"net"
	"testing"
)

func Dial(t *testing.T) net.Conn {
	conn, err := net.Dial("tcp", ":4200")
	if err != nil {
		t.Fatal(err)
	}
	return conn
}
