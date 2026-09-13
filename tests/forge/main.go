// Command forge is the adversary, and it exists so that tests/test-trusted-set.sh
// can assert what the STATION refuses rather than what our own CLI declines to
// send.
//
// `heliograph trust` will not author a change that revokes an anchor: it applies
// every change to its own copy first, and Apply refuses. Going through it would
// therefore assert that our tooling is polite, which is not the claim. The claim
// is that a document arriving at a machine nobody can log into, correctly signed
// by a key that machine trusts, still cannot move the anchor.
//
// So this holds a secret key and writes whatever it is told to - exactly the
// position somebody with a stolen key would be in.
//
// IT IS A TEST FIXTURE AND IS NAMED AS ONE in the allowlist of
// internal/trust/controlplane_test.go. It is under tests/, it is not a cmd/, and
// nothing installs it.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/dbhq-uk/heliograph/internal/seal"
	"github.com/dbhq-uk/heliograph/internal/trust"
)

func main() {
	idf := flag.String("identity", "", "")
	author := flag.String("author", "", "")
	op := flag.String("op", "", "")
	name := flag.String("name", "", "")
	key := flag.String("key", "", "")
	est := flag.String("estate", "", "")
	stn := flag.String("station", "", "")
	serial := flag.Uint64("serial", 0, "")
	prev := flag.String("prev", "", "")
	flag.Parse()

	id, err := seal.LoadIdentityFile(*idf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	subject, err := seal.DecodePublic(*key)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	c := trust.Change{
		Version: trust.Version, Estate: *est, Station: *stn,
		Serial: *serial, Prev: *prev, UTC: time.Now().UTC().Format(time.RFC3339),
		Op: *op, Name: *name, Subject: subject, Author: *author,
	}
	c, err = trust.SignChange(id, c)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(c.Marshal())
}
