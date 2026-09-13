package main

import (
	"flag"
	"io"
	"testing"
)

// A public identity is base64url, so about one in sixty-four of them begins
// with a hyphen. The shared parse() interleaves flags and positionals on
// purpose - `heliograph send step -e estate` has to work - and that is exactly
// what makes such a key indistinguishable from a flag:
//
//	$ heliograph trust add alice -62pYcHnbZjMvfPoxMQH6DKNYM1RLOAdpnUSpOyE0hC...
//	flag provided but not defined: -62pYcHnbZjMvfPoxMQH6DKNYM1RLOAdpnUSpOyE0hC...
//
// Found by driving the real command in a smoke test rather than by reading it.
// The failure lands on somebody enrolling a colleague, once every sixty-four
// colleagues, with an error that names their key as though it were a typo.
func TestATrustKeyMayBeginWithAHyphen(t *testing.T) {
	const key = "-62pYcHnbZjMvfPoxMQH6DKNYM1RLOAdpnUSpOyE0hC0daV6lE6vYLdow7oALjYDviK1Bc8MEQI4jMr6ht0PUA"

	cases := []struct {
		what   string
		args   []string
		want   []string
		estate string
		note   string
	}{
		{"a key beginning with a hyphen", []string{"alice", key}, []string{"alice", key}, "", ""},
		{"with a flag before it", []string{"-e", "acme", "alice", key}, []string{"alice", key}, "acme", ""},
		{"with a flag after it", []string{"alice", key, "-e", "acme"}, []string{"alice", key}, "acme", ""},
		{"with --flag=value form", []string{"--estate=acme", "alice", key}, []string{"alice", key}, "acme", ""},
		{"with a note that has spaces", []string{"alice", key, "--note", "for the audit"}, []string{"alice", key}, "", "for the audit"},
		{"an explicit -- still works", []string{"--", "alice", key}, []string{"alice", key}, "", ""},
		{"a revoke, which takes one positional", []string{"-e", "acme", "carol"}, []string{"carol"}, "acme", ""},
	}

	for _, tc := range cases {
		fs := flag.NewFlagSet("trust", flag.ContinueOnError)
		fs.SetOutput(io.Discard) // no usage spew on a failure
		estate := estateFlag(fs)
		note := fs.String("note", "", "")
		got, err := parseTrustArgs(fs, tc.args)
		if err != nil {
			t.Errorf("%s: %v", tc.what, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %q, want %q", tc.what, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: positional %d is %q, want %q", tc.what, i, got[i], tc.want[i])
			}
		}
		if *estate != tc.estate {
			t.Errorf("%s: estate is %q, want %q", tc.what, *estate, tc.estate)
		}
		if *note != tc.note {
			t.Errorf("%s: note is %q, want %q", tc.what, *note, tc.note)
		}
	}
}

// An unknown flag must still be an error rather than silently becoming a
// positional. Otherwise `heliograph trust add alice --nte "typo"` enrols
// somebody called `--nte`.
func TestAnUnknownFlagIsStillRefused(t *testing.T) {
	fs := flag.NewFlagSet("trust", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	_ = estateFlag(fs)

	// A single-dash token that is not a registered flag is treated as a
	// positional, which is what makes a hyphenated key work. The protection
	// against a mistyped flag is therefore the ARITY check in the command: two
	// positionals for add, one for revoke.
	got, err := parseTrustArgs(fs, []string{"alice", "KEY", "--nte", "typo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 2 {
		t.Error("a mistyped flag was silently dropped rather than becoming a surplus positional the arity check catches")
	}
	if len(got) != 4 {
		t.Errorf("got %q: a mistyped flag and its value should both survive as positionals, so the arity check refuses the command", got)
	}
}
