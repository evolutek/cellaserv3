// From https://github.com/prometheus/prometheus/blob/master/util/testutil/testing.go
// under MIT License (MIT)

package testutil

import "reflect"

// This package is imported by non-test code and therefore cannot import the
// testing package, which has side effects such as adding flags. Hence we use an
// interface to testing.{T,B}.
type TB interface {
	Helper()
	Fatalf(string, ...interface{})
}

// Assert fails the test if the condition is false.
func Assert(tb TB, condition bool, format string, a ...interface{}) {
	tb.Helper()
	if !condition {
		tb.Fatalf("\033[31m"+format+"\033[39m\n", a...)
	}
}

// Ok fails the test if an err is not nil.
func Ok(tb TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatalf("\033[31munexpected error: %v\033[39m\n", err)
	}
}

// NotOk fails the test if an err is nil.
func NotOk(tb TB, err error, format string, a ...interface{}) {
	tb.Helper()
	if err == nil {
		if len(a) != 0 {
			tb.Fatalf("\033[31m"+format+": expected error, got none\033[39m", a...)
		}
		tb.Fatalf("\033[31mexpected error, got none\033[39m")
	}
}

// Equals fails the test if exp is not equal to act.
func Equals(tb TB, exp, act interface{}) {
	tb.Helper()
	if !reflect.DeepEqual(exp, act) {
		tb.Fatalf("\033[31m\nexp: %#v\n\ngot: %#v\033[39m\n", exp, act)
	}
}
