package main

import (
	"os"
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/transport"
)

// The unit tests exercise the transport. These exercise the CLI's own wiring,
// which is where the interesting mistakes are: an estate that saves fields the
// loader does not read, a URL mangled into a path, credentials expected from a
// place nobody sets.

// filepath.Abs on an endpoint turns https://s3.example.com into
// $PWD/https:/s3.example.com. Silently: the estate saves, and every later
// command reports it cannot reach a host with a name nobody typed.
func TestObjStoreEndpointIsNotTurnedIntoAPath(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HELIOGRAPH_S3_ACCESS_KEY", "AKIDEXAMPLE")
	t.Setenv("HELIOGRAPH_S3_SECRET_KEY", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")

	const endpoint = "https://s3.eu-west-2.amazonaws.com"
	err := cmdInit([]string{"payments", "--transport", "objstore",
		"--dir", endpoint, "--bucket", "heliograph-transport", "--scope", "net"})
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	e, err := estate.Load("payments")
	if err != nil {
		t.Fatal(err)
	}
	if e.Dir != endpoint {
		t.Errorf("the endpoint was rewritten: %q, want %q", e.Dir, endpoint)
	}
	if e.Bucket != "heliograph-transport" || e.Scope != "net" {
		t.Errorf("estate did not record the bucket and lane: %+v", e)
	}
}

// Everything the estate file needs to reconstruct the transport must be in it,
// and everything secret must not. An estate that saves a bucket but no lane
// reopens as a different location.
func TestObjStoreEstateRoundTripsThroughDisk(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HELIOGRAPH_S3_ACCESS_KEY", "AKIDEXAMPLE")
	t.Setenv("HELIOGRAPH_S3_SECRET_KEY", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")

	if err := cmdInit([]string{"payments", "--transport", "objstore",
		"--dir", "https://s3.example.com", "--bucket", "b", "--scope", "net",
		"--prefix", "probe", "--region", "eu-west-2"}); err != nil {
		t.Fatal(err)
	}

	op, err := open("payments")
	if err != nil {
		t.Fatalf("an estate that saved could not be reopened: %v", err)
	}
	if !strings.Contains(op.Describe(), "b") || !strings.Contains(op.Describe(), "net") {
		t.Errorf("reopened transport lost the bucket or lane: %s", op.Describe())
	}
}

// The keys must never reach the estate file. It is on disk, gets copied between
// machines and ends up in backups; a secret in it is a secret in all three.
func TestObjStoreSecretsNeverReachTheEstateFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	const secret = "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY"
	t.Setenv("HELIOGRAPH_S3_ACCESS_KEY", "AKIDEXAMPLE")
	t.Setenv("HELIOGRAPH_S3_SECRET_KEY", secret)

	if err := cmdInit([]string{"payments", "--transport", "objstore",
		"--dir", "https://s3.example.com", "--bucket", "b", "--scope", "net"}); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(root + "/heliograph/estates/payments.json")
	if err != nil {
		t.Fatal(err)
	}
	body := string(b)
	if strings.Contains(body, secret) {
		t.Fatalf("the secret key was written to the estate file:\n%s", body)
	}
	if strings.Contains(body, "AKIDEXAMPLE") {
		t.Errorf("the access key was written to the estate file:\n%s", body)
	}
}

// Without credentials there is nothing to do, and saying so at init is the
// difference between one confusing error and a whole investigation set up
// against an estate that cannot work.
func TestObjStoreInitRefusesWithoutCredentials(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HELIOGRAPH_S3_ACCESS_KEY", "")
	t.Setenv("HELIOGRAPH_S3_SECRET_KEY", "")

	err := cmdInit([]string{"payments", "--transport", "objstore",
		"--dir", "https://s3.example.com", "--bucket", "b", "--scope", "net"})
	if err == nil {
		t.Fatal("an object store estate was created with no credentials")
	}
	if !strings.Contains(err.Error(), "HELIOGRAPH_S3_ACCESS_KEY") {
		t.Errorf("the error does not name the variables to set: %v", err)
	}
}

func TestObjStoreInitRequiresBucketAndLane(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HELIOGRAPH_S3_ACCESS_KEY", "AK")
	t.Setenv("HELIOGRAPH_S3_SECRET_KEY", "SK")

	for _, c := range []struct {
		name, want string
		args       []string
	}{
		{"no bucket", "--bucket", []string{"e", "--transport", "objstore",
			"--dir", "https://s3.example.com", "--scope", "net"}},
		{"no lane", "--scope", []string{"e", "--transport", "objstore",
			"--dir", "https://s3.example.com", "--bucket", "b"}},
	} {
		err := cmdInit(c.args)
		if err == nil {
			t.Errorf("%s: accepted", c.name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the error does not name %s: %v", c.name, c.want, err)
		}
	}
}

// objstore has to satisfy the same interface every other transport does. If it
// did not, the CLI above it would need a branch, and a transport that only some
// commands understand is worse than no transport.
func TestObjStoreSatisfiesTheTransportInterface(t *testing.T) {
	var _ transport.Transport = (*transport.ObjStore)(nil)
}
