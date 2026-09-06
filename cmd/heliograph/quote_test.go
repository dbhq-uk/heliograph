package main

import "testing"

// The station splits the env line the way a shell would, and the shell that
// invoked this command has already eaten the quotes: by the time
// `HOSTS="sql01 sql02"` arrives it is one argument with a space in it.
//
// Joining those with spaces produces `env: HOSTS=sql01 sql02`, which the far
// side reads as HOSTS=sql01 followed by an attempt to run `sql02`. That is a
// wrong run on a machine nobody can reach, reported as a step failure.
func TestShellQuote(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"PORTS=1433", "PORTS=1433"},
		{"HOSTS=sql01 sql02", "HOSTS='sql01 sql02'"},
		{"A=", "A="},
		{"PATTERN=a*b", "PATTERN='a*b'"},
		{"CMD=a;rm -rf /", "CMD='a;rm -rf /'"},
		{"Q=it's", `Q='it'\''s'`},
		{"URL=https://h/x?a=1&b=2", "URL='https://h/x?a=1&b=2'"},
		{"notakeyvalue", "notakeyvalue"},
	} {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

// The point is not the quoting, it is what a shell does with the result. An
// embedded single quote is the case that breaks naive implementations, and it
// breaks them by ending the quoting early.
func TestQuotedValueSurvivesAShell(t *testing.T) {
	for _, v := range []string{"sql01 sql02", "it's", "a;b", "a b'c d", "$HOME", "`id`"} {
		q := shellQuote("V=" + v)
		got := evalAssign(t, q)
		if got != v {
			t.Errorf("V=%q became %q after eval of %s", v, got, q)
		}
	}
}
