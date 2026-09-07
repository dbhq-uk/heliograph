package wire

import "strings"

// QuoteEnv re-quotes a KEY=VALUE pair whose value needs it, for the env field.
//
// The env field is verbatim shell text, and the station splits it the way a
// shell would. The shell that invoked the control side has already eaten the
// quotes: by the time `HOSTS="sql01 sql02"` arrives it is one argument with a
// space in it. Joining those with spaces produces `env: HOSTS=sql01 sql02`,
// which the far side reads as HOSTS=sql01 followed by an attempt to RUN
// `sql02`. That is a wrong run on a machine nobody can reach, reported as a
// step failure.
//
// So the quoting has to be put back. Single quotes, with the standard escape
// for an embedded one, because inside single quotes a shell interprets nothing
// at all - and this string is about to be split by one on the far side.
//
// This lives in wire, not in a command, because the env field is part of the
// document. Two copies of the code that stands between setting a variable and
// running a command is exactly the code that must not be allowed to drift.
func QuoteEnv(kv string) string {
	i := strings.IndexByte(kv, '=')
	if i < 0 {
		return kv
	}
	k, v := kv[:i], kv[i+1:]
	// An empty value is already unambiguous, and A='' reads like a mistake.
	if v == "" || !strings.ContainsAny(v, " \t\n\"'\\$`&|;<>()*?[]#~!") {
		return kv
	}
	return k + "='" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
