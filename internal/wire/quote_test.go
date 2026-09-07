package wire

import (
	"os/exec"
	"testing"
)

func TestQuoteEnv(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"PORTS=1433", "PORTS=1433"},
		{"HOSTS=sql01 sql02", "HOSTS='sql01 sql02'"},
		{"A=", "A="},
		{"PATTERN=a*b", "PATTERN='a*b'"},
		{"CMD=a;rm -rf /", "CMD='a;rm -rf /'"},
		{"Q=it's", `Q='it'\''s'`},
		{"URL=https://h/x?a=1&b=2", "URL='https://h/x?a=1&b=2'"},
		{"notakeyvalue", "notakeyvalue"},
		// The FIRST = splits, because a value may contain one.
		{"A=b=c", "A=b=c"},
		{"A=x y=z", `A='x y=z'`},
	} {
		if got := QuoteEnv(tc.in); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.in, got, tc.want)
		}
	}
}

// The point is not the quoting, it is what a shell does with the result. An
// embedded single quote is the case that breaks naive implementations, and it
// breaks them by ending the quoting early.
//
// The assignment is passed as an ARGUMENT rather than interpolated into the
// script. Interpolating it means the test's own shell eats the quotes before
// eval sees them, which is the same mistake the code under test exists to fix,
// and it makes a broken implementation pass.
func TestQuotedValueSurvivesAShell(t *testing.T) {
	for _, v := range []string{
		"sql01 sql02", "it's", "a;b", "a b'c d", "$HOME", "`id`",
		"$(id)", "a|b", "a&b", `a\b`, "a*b", "a>b", "x=y z",
	} {
		q := QuoteEnv("V=" + v)
		out, err := exec.Command("bash", "-c", `eval "$1"; printf '%s' "$V"`, "_", q).Output()
		if err != nil {
			t.Fatalf("eval %q: %v", q, err)
		}
		if string(out) != v {
			t.Errorf("V=%q became %q after eval of %s", v, out, q)
		}
	}
}
