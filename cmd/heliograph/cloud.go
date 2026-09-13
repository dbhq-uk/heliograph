package main

// The commands that talk to a hosted heliograph service.
//
// Everything else in this binary talks to a transport the customer operates.
// These four talk to somebody else's machine over the network, which is a
// different kind of thing, so they are kept in one file where the credential
// handling can be read in one sitting.
//
//	heliograph login    acquire an account credential, without pasting one
//	heliograph push     forward the spool this control node already keeps
//	heliograph rotate   replace an estate's control credential
//	heliograph init --hosted   provision an estate, so there is nothing to stand up
//
// The contracts are on heliograph-io/heliograph-cloud: #11 for sign-in and
// provisioning, #16 for the upload protocol, #19 for the record, #10 for the
// refusal taxonomy.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/cloud"
	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/seal"
)

// serviceURL resolves which service to talk to.
//
// THERE IS NO DEFAULT, and the refusal says so plainly rather than failing at
// a hostname nobody typed. A URL compiled into a released binary is a claim
// that something answers there, and a binary in the field outlives the claim.
func serviceURL(flagValue string) (*cloud.Service, error) {
	raw := flagValue
	if raw == "" {
		raw = os.Getenv("HELIOGRAPH_CLOUD_URL")
	}
	if raw == "" {
		return nil, fmt.Errorf("no service URL. This build has no hosted service compiled into it,\n" +
			"  because a hostname in a released binary is a promise that outlives the binary.\n" +
			"  Pass --service <https url>, or set HELIOGRAPH_CLOUD_URL")
	}
	return cloud.NewService(raw)
}

// serviceFlag registers the same spelling on every command that needs one.
func serviceFlag(fs *flag.FlagSet) *string {
	return fs.String("service", "", "the hosted service (default: HELIOGRAPH_CLOUD_URL)")
}

func clientLabel() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "an unnamed host"
	}
	return fmt.Sprintf("heliograph %s on %s (%s)", version, host, runtime.GOOS)
}

// cmdLogin acquires an account credential through a browser.
//
// NOTHING IS PASTED, and the step being removed is a step rather than a
// keystroke. A credential pasted into a terminal is a credential in a shell
// history, a scrollback buffer and whatever the terminal emulator keeps - and
// the reason it was pasted at all is that somebody had to be SHOWN it, so it
// was on a screen and in a clipboard too. Here the CLI shows a short code that
// opens nothing, the browser carries the authentication, and the credential
// comes back over the connection the CLI already opened.
func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	svc := serviceFlag(fs)
	status := fs.Bool("status", false, "what is stored here, without changing anything")
	logout := fs.Bool("logout", false, "forget the credential for this service")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *status {
		return loginStatus(*svc)
	}

	s, err := serviceURL(*svc)
	if err != nil {
		return err
	}
	if *logout {
		creds, cerr := cloud.LoadCredentials()
		if cerr != nil {
			return cerr
		}
		if !creds.ForgetAccount(s.Base) {
			fmt.Printf("nothing stored here for %s\n", s.Base)
			return nil
		}
		if err := creds.Save(); err != nil {
			return err
		}
		fmt.Printf("forgotten: the credential for %s is no longer on this machine\n", s.Base)
		fmt.Println("  it is not revoked by this. Revoke it in the console if it may have been seen")
		return nil
	}

	_, err = cloud.Login(s, clientLabel(), func(line string) { fmt.Println(line) }, time.Sleep)
	return err
}

func loginStatus(svc string) error {
	creds, err := cloud.LoadCredentials()
	if err != nil {
		return err
	}
	path, _ := cloud.CredentialsPath()
	if len(creds.Accounts) == 0 && len(creds.Estates) == 0 {
		fmt.Printf("nothing is stored at %s\n", path)
		fmt.Println("  run `heliograph login --service <url>` to sign in")
		return nil
	}
	fmt.Printf("%s\n\n", path)
	// THE VALUES ARE NEVER PRINTED, here least of all: this is the command
	// somebody runs while sharing a screen with the person helping them.
	for service, a := range creds.Accounts {
		who := a.Label
		if who == "" {
			who = a.AccountID
		}
		fmt.Printf("account  %-40s %s", service, who)
		if a.Obtained != "" {
			fmt.Printf("  (since %s)", a.Obtained)
		}
		fmt.Println()
	}
	for name, e := range creds.Estates {
		fmt.Printf("estate   %-40s %s on %s\n", name, e.Estate, e.Service)
	}
	if svc != "" {
		if _, ok := creds.Account(svc); !ok {
			fmt.Printf("\nnothing stored for %s\n", svc)
		}
	}
	return nil
}

// relaySpool is where a relay estate's spool lives. It has to agree with
// `open()` below, which is why it is one function rather than two joins.
func relaySpool(e estate.Estate) (string, error) {
	dir, err := estate.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, e.Name+".relay"), nil
}

// estateAuth finds the credential for one estate.
//
// The environment wins, because that is the form a pipeline injects and a
// pipeline must not be overridden by whatever a person signed in as. The store
// is the fallback, and it is what `login` and `init --hosted` fill in.
// THE CREDENTIAL AND THE SERVICE URL ARE LOOKED UP SEPARATELY, and that is a
// correction the first round trip forced. An earlier version returned no URL
// when the credential came from the environment, so
// `HELIOGRAPH_CLOUD_TOKEN=... heliograph push` failed with "no service URL" -
// which reads as a missing flag and is actually a lookup this side skipped. The
// two answers are independent: overriding one has never meant discarding the
// other.
func estateAuth(e estate.Estate) (cloud.Auth, string, error) {
	var stored cloud.EstateCredential
	var have bool
	if creds, err := cloud.LoadCredentials(); err == nil {
		stored, have = creds.Estate(e.Name)
	} else {
		return cloud.Auth{}, "", err
	}
	if tok := os.Getenv("HELIOGRAPH_CLOUD_TOKEN"); tok != "" {
		return cloud.Control(tok), stored.Service, nil
	}
	if !have {
		return cloud.Auth{}, "", fmt.Errorf(
			"estate %q has no hosted credential on this machine.\n"+
				"  `heliograph init %s --transport relay --hosted` provisions one, or set\n"+
				"  HELIOGRAPH_CLOUD_TOKEN to the estate's control credential", e.Name, e.Name)
	}
	return cloud.Control(stored.Token), stored.Service, nil
}

// cmdPush forwards the spool.
//
// IT READS THE SPOOL AND NEVER WRITES TO IT. The spool is the customer's own
// archive - the only copy of every log this control node has collected, because
// a relay deletes on collection - and forwarding a copy is not a licence to
// alter it. It also never opens a transport, so there is no code path from here
// to a station's request queue at all.
func cmdPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	name := estateFlag(fs)
	svc := serviceFlag(fs)
	dryRun := fs.Bool("dry-run", false, "say what would be sent, and send nothing")
	if err := fs.Parse(args); err != nil {
		return err
	}

	e, err := resolve(*name)
	if err != nil {
		return err
	}
	if e.Transport != "relay" {
		return fmt.Errorf("estate %q uses the %s transport, which keeps its logs where it put them.\n"+
			"  `push` forwards the spool a relay estate has to keep because a relay deletes on\n"+
			"  collection. The other transports reach the archive by their own routes",
			e.Name, e.Transport)
	}

	spoolDir, err := relaySpool(e)
	if err != nil {
		return err
	}
	sp, err := cloud.Read(spoolDir)
	if err != nil {
		return err
	}
	runs, unpaired := sp.Runs()

	if *dryRun {
		fmt.Printf("spool: %s\n", spoolDir)
		fmt.Printf("%d run(s), %d status document(s), %d body/bodies\n",
			len(runs), len(sp.Statuses), len(sp.Logs))
		for _, r := range runs {
			body := "no body collected"
			if r.Body != nil {
				body = fmt.Sprintf("%s, %d bytes", r.Body.Name, r.Body.Bytes)
			}
			fmt.Printf("  %-32s %d status(es)  %s\n", r.RunID, len(r.Statuses), body)
		}
		for _, l := range unpaired {
			fmt.Printf("  not sendable: %s - no collected status document names it\n", l.Name)
		}
		for _, g := range sp.Gaps() {
			fmt.Printf("  %s\n", g)
		}
		fmt.Println("\nnothing was sent (--dry-run)")
		return nil
	}

	auth, storedService, err := estateAuth(e)
	if err != nil {
		return err
	}
	target := *svc
	if target == "" && os.Getenv("HELIOGRAPH_CLOUD_URL") == "" {
		target = storedService
	}
	s, err := serviceURL(target)
	if err != nil {
		return err
	}

	res, perr := s.Push(cloud.Target{Station: e.Scope, Auth: auth}, sp)
	// PRINTED WHETHER IT SUCCEEDED OR NOT. A push that reported only its
	// successes would be the tooling this project's own rule forbids: a failed
	// push never loses a log, and what it did manage is what somebody needs in
	// order to decide what to do next.
	for _, line := range res.Summary() {
		fmt.Println(line)
	}
	if perr != nil {
		return perr
	}
	// SAID ONCE, WHERE SOMEBODY IS LOOKING AT THE ARCHIVE. A relay-only estate
	// gets the record and the history and does not get gone-quiet alerting,
	// because nothing reaches the service while this machine is closed.
	if len(res.Uploaded) > 0 {
		fmt.Println()
		fmt.Println("This estate reaches the service only when this control node pushes, so the")
		fmt.Println("archive is as current as the last push and nothing can alert on a station")
		fmt.Println("that has gone quiet. A git transport alongside is what closes that.")
	}
	return nil
}

// cmdRotate replaces an estate's control credential.
//
// THE WARNING IS PRINTED BEFORE THE QUESTION, not after the answer. Rotating a
// control credential is cheap because it lives on this machine; rotating a
// STATION credential is not a rotation at all, because it lives on a machine
// nobody here can reach, in front of somebody with their own schedule. Offering
// the two as if they were the same action is how a station gets stranded, and
// `relay.md` already records the version of this trap that cost an estate its
// tokens.
func cmdRotate(args []string) error {
	fs := flag.NewFlagSet("rotate", flag.ExitOnError)
	name := estateFlag(fs)
	svc := serviceFlag(fs)
	yes := fs.Bool("yes", false, "do it, having read the warning above")
	if err := fs.Parse(args); err != nil {
		return err
	}
	e, err := resolve(*name)
	if err != nil {
		return err
	}
	creds, err := cloud.LoadCredentials()
	if err != nil {
		return err
	}
	stored, ok := creds.Estate(e.Name)
	if !ok {
		return fmt.Errorf("estate %q has no hosted credential on this machine, so there is nothing here to rotate", e.Name)
	}

	fmt.Println(cloud.RotationWarning())
	if !*yes {
		fmt.Println()
		fmt.Printf("Nothing has been rotated. Run `heliograph rotate -e %s --yes` to go ahead.\n", e.Name)
		return nil
	}

	target := *svc
	if target == "" && os.Getenv("HELIOGRAPH_CLOUD_URL") == "" {
		target = stored.Service
	}
	s, err := serviceURL(target)
	if err != nil {
		return err
	}
	rot, err := s.RotateControlToken(cloud.Control(stored.Token), stored.Estate)
	if err != nil {
		return err
	}
	stored.Token = rot.ControlToken
	stored.Obtained = time.Now().UTC().Format(time.RFC3339)
	creds.SetEstate(e.Name, stored)
	if err := creds.Save(); err != nil {
		// The old credential has already stopped working, so this is the worst
		// moment to be vague about what happened.
		return fmt.Errorf("the credential was rotated and could not be written here: %w.\n"+
			"  The previous one has already stopped working. Rotate again from the console", err)
	}
	fmt.Println()
	if rot.Says != "" {
		fmt.Println(rot.Says)
	}
	if rot.StationTokens != "" {
		fmt.Println(rot.StationTokens)
	}
	fmt.Printf("estate %s: the new control credential is stored here and was not printed\n", e.Name)
	return nil
}

// hostedProvision is `init --hosted`: an estate with nothing to stand up.
//
// The step this removes is the one heliograph-io/heliograph-cloud#11 names:
// write a Docker command, generate two tokens by hand, keep them somewhere you
// can read back, and put something in front that terminates TLS - all before
// seeing the product work once. None of that is here. The customer terminates
// no TLS and runs no container, because both ends dial out over ordinary HTTPS
// to something somebody else operates.
func hostedProvision(name, svcFlag, scope string) error {
	s, err := serviceURL(svcFlag)
	if err != nil {
		return err
	}
	creds, err := cloud.LoadCredentials()
	if err != nil {
		return err
	}
	account, ok := creds.Account(s.Base)
	if !ok {
		return fmt.Errorf("not signed in to %s.\n  Run `heliograph login --service %s` first: it needs no token pasted",
			s.Base, s.Base)
	}

	prov, err := s.ProvisionEstate(cloud.Control(account.Token), name)
	if err != nil {
		return err
	}

	// THE IDENTITY IS THIS SIDE'S AND IS MADE HERE, exactly as it is for a
	// hand-configured relay estate. The service provisions routing and
	// credentials; it never sees a key that can open or sign anything.
	keys, err := estate.KeyDir()
	if err != nil {
		return err
	}
	idPath := filepath.Join(keys, name+".identity.json")
	if _, statErr := os.Stat(idPath); statErr != nil {
		id, genErr := sealGenerate()
		if genErr != nil {
			return genErr
		}
		if err := sealWriteIdentity(idPath, id); err != nil {
			return err
		}
	}
	me, err := sealLoadIdentity(idPath)
	if err != nil {
		return err
	}

	// STORED BEFORE ANYTHING IS PRINTED. Both credentials are issued once and
	// neither is shown again, so a crash between the call and the write loses
	// an estate.
	creds.SetEstate(name, cloud.EstateCredential{
		Service: s.Base, Estate: prov.Estate, Token: prov.ControlToken,
		Obtained: time.Now().UTC().Format(time.RFC3339),
	})
	if err := creds.Save(); err != nil {
		return fmt.Errorf("estate %s was provisioned and its credential could not be written: %w.\n"+
			"  It is issued once and cannot be read back", prov.Estate, err)
	}

	station := scope
	if station == "" {
		station = name
	}
	e := estate.Estate{
		Name: name, Transport: "relay", Dir: s.Base, Scope: station,
		RelayEstate: prov.Estate, Identity: idPath,
	}
	if err := e.Save(); err != nil {
		return err
	}

	fmt.Printf("estate %s -> %s, estate %s, station %s\n", name, s.Base, prov.Estate, station)
	fmt.Printf("  identity: %s\n", idPath)
	fmt.Printf("  fingerprint: %s\n", me.Public().Fingerprint())
	fmt.Println("  control credential: stored here, mode 600, and not printed")
	if prov.Says != "" {
		fmt.Println()
		fmt.Println(prov.Says)
	}
	fmt.Println()
	fmt.Println("Send this to whoever can log into the machine:")
	fmt.Println()
	fmt.Println("  " + prov.PlantingLine)
	if prov.EnrolmentKeyExpiresAt != "" {
		fmt.Printf("\n  The enrolment key in that line expires at %s, and it is shown once.\n",
			prov.EnrolmentKeyExpiresAt)
	}
	fmt.Println()
	fmt.Println("  Sending it over a channel they already trust is the only step here a")
	fmt.Println("  machine cannot do for you. That was true before this was hosted and it")
	fmt.Println("  is still true.")
	if prov.Next != "" {
		fmt.Println()
		fmt.Println(prov.Next)
	}
	return nil
}

// reportRefusal prints a service refusal the way heliograph-io/heliograph-cloud#10
// asks for, and it is used by `doctor` rather than being inlined there.
//
// BRANCHING IS ON `cause` AND NEVER ON THE STATUS CODE. 402 is both an
// exhausted allowance and a spend cap; 503 is both a full queue and our own
// authoriser being down, and only one of those is the operator's problem. A
// doctor that read the second as an authentication failure would send somebody
// to rotate a credential during somebody else's incident.
func reportRefusal(r *cloud.Refusal) {
	fmt.Printf("FAIL      %s\n", r.Error())
	switch {
	case r.Credential():
		fmt.Println("          the credential this estate holds was refused.")
		fmt.Printf("          `heliograph rotate` replaces it, or `heliograph login --status` shows what is stored\n")
	case r.Exhaustion():
		// The dimension is the service's, and the remedy differs by it, so the
		// sentence is the service's too. Composing one here would say "no new
		// command was admitted" about a station limit, where it is false.
		fmt.Println("          nothing was lost: what the service has already accepted still delivers.")
	case r.Cause == "rate-limited":
		// NOT an allowance problem. The client has lost nothing.
		if r.RetryAfterSeconds > 0 {
			fmt.Printf("          nothing was lost and nothing is exhausted. Try again in %d seconds\n",
				r.RetryAfterSeconds)
		} else {
			fmt.Println("          nothing was lost and nothing is exhausted. Try again shortly")
		}
	case r.Ours():
		fmt.Println("          this is the service's own problem and not this estate's credential.")
		fmt.Println("          Rotating anything now would change nothing. Check the service's status")
	case !r.Known():
		// PRINTED, NOT COLLAPSED. The same rule as an unrecognised run state:
		// a reader can look up a word they have not seen, and cannot recover
		// one this build replaced with "error".
		fmt.Printf("          this build does not know the cause %q, so it is printed as the service sent it\n", r.Cause)
	}
}

// cloudDoctor adds the hosted half to `heliograph doctor`.
//
// It reports and never changes anything, the same as the rest of that command.
// Returns the number of blocking problems it found.
func cloudDoctor(e estate.Estate) int {
	creds, err := cloud.LoadCredentials()
	if err != nil {
		fmt.Printf("FAIL      the credential store is not readable: %v\n", err)
		return 1
	}
	stored, ok := creds.Estate(e.Name)
	if !ok {
		// Not a failure. An estate configured by hand against a self-hosted
		// relay has no hosted credential and is not missing one.
		return 0
	}
	s, err := cloud.NewService(stored.Service)
	if err != nil {
		fmt.Printf("FAIL      the stored service URL for estate %s is not usable: %v\n", e.Name, err)
		return 1
	}
	timing, err := s.Timing(cloud.Control(stored.Token), stored.Estate)
	if err != nil {
		if r, isRefusal := cloud.AsRefusal(err); isRefusal {
			reportRefusal(r)
			return 1
		}
		fmt.Printf("FAIL      cannot reach the hosted service at %s: %v\n", s.Base, err)
		fmt.Printf("          the transport above is a separate question: a relay estate can be\n")
		fmt.Printf("          working perfectly while the archive is unreachable\n")
		return 1
	}
	fmt.Printf("ok        the hosted service at %s answers this estate's credential\n", s.Base)
	fmt.Printf("          %s\n", timing.Line())
	return 0
}

// The seal helpers are indirected through these three so that this file reads
// as one story. They are the same calls `init` already makes for a relay
// estate; nothing here is a second way of making a key.
var (
	sealGenerate      = sealGenerateImpl
	sealWriteIdentity = sealWriteIdentityImpl
	sealLoadIdentity  = sealLoadIdentityImpl
)

// usageForCloud is folded into the top-level usage string, kept here so the
// four commands are documented beside themselves.
var usageForCloud = strings.Join([]string{
	"  heliograph login                          sign in to a hosted service, with nothing to paste",
	"      [--service <url>] [--status] [--logout]",
	"  heliograph push                           forward this control node's spool to the archive",
	"      [-e <estate>] [--dry-run]",
	"  heliograph rotate                         replace an estate's control credential",
	"      [-e <estate>] [--yes]",
}, "\n")

func sealGenerateImpl() (*seal.Identity, error) { return seal.Generate() }

func sealWriteIdentityImpl(path string, id *seal.Identity) error {
	return seal.WriteIdentityFile(path, id)
}

func sealLoadIdentityImpl(path string) (*seal.Identity, error) {
	return seal.LoadIdentityFile(path)
}
