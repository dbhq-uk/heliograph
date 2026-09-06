// Command heliograph drives a station across a gap you cannot cross yourself.
//
// This is the control side. It publishes a request, and reads the log that
// comes back. The far side is a stock heliograph-skill station and nothing
// here requires a change to it.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/logfile"
	"github.com/dbhq-uk/heliograph/internal/plant"

	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/transport"
	"github.com/dbhq-uk/heliograph/internal/wire"
)

// version is set at build time with -ldflags. "dev" means somebody built this
// from source, which is worth saying rather than printing a version that is
// not one.
var version = "dev"

const usage = `heliograph - run things on a machine you cannot log into

  heliograph init <estate> --dir <path>     remember a transport repo by name
  heliograph estates                        what is configured here
  heliograph plant                          what to send the operator
  heliograph send <step> [K=V ...]          publish a request, and return
  heliograph status                         what the station is doing now
  heliograph watch                          follow a run until it ends
  heliograph logs                           list the captured logs
  heliograph logs <name>                    print one, whole
  heliograph logs --last --gaps             where the last run stalled
  heliograph doctor                         will this work from here, in full
  heliograph version

Common flags:
  -e, --estate <name>   which estate (default: the only one, if there is one)

A step is a name registered in the station's run.sh, or a path to a step file.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = cmdInit(os.Args[2:])
	case "estates":
		err = cmdEstates()
	case "plant":
		err = cmdPlant(os.Args[2:])
	case "send":
		err = cmdSend(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "logs":
		err = cmdLogs(os.Args[2:])
	case "watch":
		err = cmdWatch(os.Args[2:])
	case "check", "doctor":
		err = cmdDoctor(os.Args[2:])
	case "version", "--version":
		fmt.Printf("heliograph %s\n", version)
		return
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "heliograph: "+err.Error())
		os.Exit(1)
	}
}

// resolve finds the estate to act on.
//
// With exactly one configured, naming it every time is friction for no gain.
// With several, it must be named: sending a request to the wrong estate runs a
// command on the wrong machine, and that is not recoverable by apologising.
func resolve(name string) (estate.Estate, error) {
	if name != "" {
		return estate.Load(name)
	}
	names, err := estate.List()
	if err != nil {
		return estate.Estate{}, err
	}
	switch len(names) {
	case 0:
		return estate.Estate{}, fmt.Errorf("no estates configured: run `heliograph init <name> --dir <path>`")
	case 1:
		return estate.Load(names[0])
	default:
		return estate.Estate{}, fmt.Errorf("several estates configured (%s): name one with --estate",
			strings.Join(names, ", "))
	}
}

// opened is what a command needs: the transport, plus the two facts every
// command prints. Returning the interface rather than *Git is what stops each
// command growing a switch of its own.
type opened struct {
	estate.Estate
	transport.Transport
	Dir    string
	Scope  string
	Origin string // what the far side would clone; empty where that has no meaning
}

func open(name string) (opened, error) {
	e, err := resolve(name)
	if err != nil {
		return opened{}, err
	}
	switch e.Transport {
	case "git":
		g, err := transport.NewGit(e.Dir)
		if err != nil {
			return opened{}, err
		}
		url, _ := g.RemoteURL()
		return opened{Estate: e, Transport: g, Dir: g.Dir(), Scope: g.Branch(), Origin: url}, nil
	case "share":
		s, err := transport.NewShare(e.Dir, e.Scope)
		if err != nil {
			return opened{}, err
		}
		return opened{Estate: e, Transport: s, Dir: s.Dir(), Scope: s.Branch()}, nil
	case "bundle":
		b, err := transport.NewBundle(e.Dir)
		if err != nil {
			return opened{}, err
		}
		return opened{Estate: e, Transport: b, Dir: b.Dir(), Scope: b.Branch()}, nil
	default:
		return opened{}, fmt.Errorf("estate %q names transport %q, which this build does not know", e.Name, e.Transport)
	}
}

// parse handles flags and positional arguments in any order.
//
// Go's flag package stops at the first non-flag argument, so `init e2e --dir x`
// silently never sees --dir and reports a usage error about an argument that
// was right there. Nobody types flags-before-positionals by instinct, and a
// CLI that insists on it teaches that lesson one confusing error at a time.
//
// Returns the positional arguments in the order they were given.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// estateFlag registers the same two spellings on every subcommand.
func estateFlag(fs *flag.FlagSet) *string {
	s := fs.String("estate", "", "which estate")
	fs.StringVar(s, "e", "", "which estate")
	return s
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "the working clone, share directory, or bundle directory")
	kind := fs.String("transport", "git", "git | share | bundle")
	scopeFlag := fs.String("scope", "", "with --transport share: one directory per investigation")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return fmt.Errorf("usage: heliograph init <estate> --dir <path>")
	}
	name := pos[0]
	if *dir == "" {
		return fmt.Errorf("--dir is required: it is the clone this estate writes to")
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		return err
	}

	// Attach BEFORE saving. An estate that names a directory which is not a
	// usable transport is worse than no estate at all: it fails later, from a
	// command that had every reason to expect it to work.
	var tp transport.Transport
	var scope string
	switch *kind {
	case "git":
		g, err := transport.NewGit(abs)
		if err != nil {
			return err
		}
		tp, scope = g, g.Branch()
	case "share":
		if *scopeFlag == "" {
			return fmt.Errorf("--scope is required for a share: one directory per investigation, so two do not overwrite each other")
		}
		sh, err := transport.NewShare(abs, *scopeFlag)
		if err != nil {
			return err
		}
		tp, scope = sh, sh.Branch()
	case "bundle":
		b, err := transport.NewBundle(abs)
		if err != nil {
			return err
		}
		tp, scope = b, b.Branch()
	default:
		return fmt.Errorf("unknown transport %q: this build knows git, share and bundle", *kind)
	}

	e := estate.Estate{Name: name, Transport: *kind, Dir: abs, Branch: scope, Scope: *scopeFlag}
	if err := e.Save(); err != nil {
		return err
	}
	fmt.Printf("estate %s -> %s on %s\n", name, abs, scope)
	fmt.Printf("  %s\n", tp.Describe())
	if err := tp.Check(); err != nil {
		// Saved anyway. Knowing the estate is configured but unreachable is
		// more useful than refusing to record it, and the reason is printed.
		fmt.Printf("  warn: %v\n", err)
	}
	return nil
}

func cmdEstates() error {
	names, err := estate.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("no estates configured")
		return nil
	}
	for _, n := range names {
		e, err := estate.Load(n)
		if err != nil {
			fmt.Printf("%-16s (unreadable: %v)\n", n, err)
			continue
		}
		fmt.Printf("%-16s %-7s %s\n", e.Name, e.Transport, e.Dir)
	}
	return nil
}

func cmdSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ExitOnError)
	name := estateFlag(fs)
	note := fs.String("note", "", "free text for the next human")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) < 1 {
		return fmt.Errorf("usage: heliograph send <step> [KEY=VALUE ...]")
	}
	step := pos[0]

	// Everything after the step is environment for the run, passed verbatim.
	// The station splits it the way a shell would.
	var env []string
	for _, a := range pos[1:] {
		if !strings.Contains(a, "=") {
			return fmt.Errorf("%q is not KEY=VALUE: environment for the run goes after the step name", a)
		}
		env = append(env, shellQuote(a))
	}

	op, err2 := open(*name)
	if err2 != nil {
		return err2
	}
	req := wire.Request{
		Version: wire.Version,
		ID:      wire.NewID(step, time.Now()),
		Step:    step,
		Env:     strings.Join(env, " "),
		Note:    *note,
	}
	if err := op.PutRequest(req); err != nil {
		return err
	}
	fmt.Printf("sent %s\n", req.ID)
	fmt.Printf("  step: %s\n", step)
	if req.Env != "" {
		fmt.Printf("  env:  %s\n", req.Env)
	}
	fmt.Println("  the station picks this up within its poll interval. `heliograph status` to follow it.")
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	name := estateFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	op, err := open(*name)
	if err != nil {
		return err
	}
	s, err := op.FetchStatus()
	if err != nil {
		return err
	}
	if s.State == "" {
		// Distinguish "never woke up" from "working". Without this they look
		// identical until a log appears, which can be an hour.
		fmt.Println("the station has published no status: it may not have started yet")
		return nil
	}
	fmt.Printf("state:    %s\n", s.State)
	printIf("id:      ", s.ID)
	printIf("step:    ", s.Step)
	printIf("host:    ", s.Host)
	printIf("started: ", s.Started)
	printIf("progress:", s.Progress)
	printIf("last:    ", s.Last)
	printIf("finished:", s.Finished)
	printIf("exit:    ", s.Exit)
	printIf("log:     ", s.Log)
	if s.Refused() {
		// A refusal names a flag somebody has to pass. Saying so here saves
		// the round trip that would otherwise be spent looking for a broken
		// step that is not broken.
		fmt.Println()
		fmt.Println("refused: the station would not run this. Its reason is above.")
		fmt.Println("  an action step needs the station started with --allow-actions,")
		fmt.Println("  and the request to carry CONFIRM=yes.")
	}
	return nil
}

func printIf(label, v string) {
	if v != "" {
		fmt.Printf("%s %s\n", label, v)
	}
}

func cmdLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	name := estateFlag(fs)
	last := fs.Bool("last", false, "the most recent log")
	gaps := fs.Bool("gaps", false, "where it stalled, instead of the whole log")
	min := fs.Duration("min", 10*time.Second, "with --gaps, the shortest interval worth reporting")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	op, err := open(*name)
	if err != nil {
		return err
	}

	if len(pos) == 0 && !*last {
		names, err := op.ListLogs()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Println("no logs yet")
			return nil
		}
		for _, n := range names {
			fmt.Println(n)
		}
		return nil
	}

	var target string
	if len(pos) > 0 {
		target = pos[0]
	}
	if *last {
		names, err := op.ListLogs()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("no logs yet")
		}
		target = names[0]
	}
	b, err := op.ReadLog(target)
	if err != nil {
		return err
	}
	if *gaps {
		return printGaps(target, b, *min)
	}
	// Whole, always. The line somebody truncates is the line they needed.
	_, err = os.Stdout.Write(b)
	return err
}

// printGaps turns the timestamp column into an answer.
//
// "Scan the timestamp column for gaps before reading the content" has always
// been a discipline somebody has to remember. It is arithmetic, and a hang
// shows up here as a gap or it does not show up at all.
func printGaps(name string, body []byte, min time.Duration) error {
	n, err := logfile.CountStamped(bytes.NewReader(body))
	if err != nil {
		return err
	}
	found, err := logfile.Gaps(bytes.NewReader(body), min)
	if err != nil {
		// A capture where every line carries the same stamp is broken, and
		// reporting "no gaps" about it would look like a clean run.
		return err
	}
	fmt.Printf("%s\n%d captured lines\n\n", name, n)
	if len(found) == 0 {
		fmt.Printf("no interval of %s or more: nothing stalled\n", min)
		return nil
	}
	fmt.Printf("%d interval(s) of %s or more, longest first.\n", len(found), min)
	fmt.Printf("Each is attributed to the line BEFORE it, which is what was running.\n\n")
	for _, g := range found {
		fmt.Println(g.Format())
	}
	return nil
}

// cmdWatch follows a run to its end.
//
// Without this the choice is polling `status` by hand or waiting blind, and
// "running for forty minutes" and "wedged" look identical from here until a
// log appears.
func cmdWatch(args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	name := estateFlag(fs)
	every := fs.Duration("interval", 10*time.Second, "how often to poll")
	timeout := fs.Duration("timeout", 0, "give up after this long (0 waits indefinitely)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	op, err := open(*name)
	if err != nil {
		return err
	}

	deadline := time.Time{}
	if *timeout > 0 {
		deadline = time.Now().Add(*timeout)
	}
	var lastLine string
	for {
		s, err := op.FetchStatus()
		if err != nil {
			// A fetch failure is a blip, not a death. The station's own loop
			// treats it that way and so does this: reporting and carrying on
			// is right for a link that flaps.
			fmt.Printf("%s  fetch failed, still watching: %v\n", stamp(), err)
		} else {
			line := s.State
			if s.Progress != "" {
				line += "  " + s.Progress
			}
			if s.Last != "" {
				line += "\n           last: " + s.Last
			}
			// Print transitions, not every poll. A watch that reprints the
			// same line every ten seconds buries the change it exists to show.
			if line != lastLine {
				fmt.Printf("%s  %s\n", stamp(), line)
				lastLine = line
			}
			if s.Done() {
				fmt.Println()
				if s.Refused() {
					fmt.Println("refused: the station would not run this.")
					fmt.Println("  an action step needs the station started with --allow-actions,")
					fmt.Println("  and the request to carry CONFIRM=yes.")
					return nil
				}
				if s.Log != "" {
					fmt.Printf("log: %s\n", s.Log)
					fmt.Printf("  heliograph logs --last          to read it\n")
					fmt.Printf("  heliograph logs --last --gaps   to see where it stalled\n")
				}
				return nil
			}
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			return fmt.Errorf("still running after %s: the run continues, this stopped watching", *timeout)
		}
		time.Sleep(*every)
	}
}

func stamp() string { return time.Now().UTC().Format("15:04:05Z") }

// cmdDoctor answers "will this work from here" and changes nothing.
//
// Every line that reports a problem also says what to do about it. A preflight
// line that names a fault without a remedy is a defect: the person reading it
// usually cannot ask anybody.
func cmdDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	name := estateFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	op, err := open(*name)
	if err != nil {
		return err
	}
	fmt.Printf("estate:   %s\n", op.Estate.Name)
	fmt.Printf("dir:      %s\n", op.Dir)
	fmt.Printf("scope:    %s\n", op.Scope)
	fmt.Printf("%s\n", op.Describe())

	problems := 0
	if err := op.Check(); err != nil {
		problems++
		fmt.Printf("FAIL      %v\n", err)
		fmt.Printf("          check that %s is correct and this account may use it\n", op.Dir)
	} else {
		fmt.Println("ok        the transport is reachable")
	}

	s, err := op.FetchStatus()
	switch {
	case err != nil:
		problems++
		fmt.Printf("FAIL      cannot read the station's status: %v\n", err)
	case s.State == "":
		// Not a failure. A station that has never run is the ordinary state
		// of a repo that was set up an hour ago, and saying so plainly beats
		// a warning that reads like something is wrong.
		fmt.Println("note      the station has published no status yet")
		fmt.Println("          it may not have been started: the operator runs ./start.sh once")
	default:
		fmt.Printf("ok        the station last published %q", s.State)
		if s.UTC != "" {
			fmt.Printf(" at %s", s.UTC)
		}
		fmt.Println()
	}

	logs, err := op.ListLogs()
	if err != nil {
		problems++
		fmt.Printf("FAIL      cannot list logs: %v\n", err)
	} else {
		fmt.Printf("ok        %d log(s) on this branch\n", len(logs))
	}

	if problems > 0 {
		return fmt.Errorf("%d blocking problem(s) above", problems)
	}
	return nil
}

// cmdPlant prints what to send the operator.
//
// Today this is prose in a chat window: somebody types out a clone URL, a
// branch and a command, and the operator retypes them. Every retyping is a
// chance to get it wrong on a machine nobody can check afterwards.
func cmdPlant(args []string) error {
	fs := flag.NewFlagSet("plant", flag.ExitOnError)
	name := estateFlag(fs)
	service := fs.Bool("service", false, "install it to survive logout, rather than run in a shell")
	script := fs.Bool("script", false, "just the commands, with no explanation around them")
	if err := fs.Parse(args); err != nil {
		return err
	}
	op, err := open(*name)
	if err != nil {
		return err
	}
	if op.Origin == "" {
		return fmt.Errorf("estate %q uses the %s transport, which has nothing for the far side to clone: plant it by carrying the toolkit there",
			op.Estate.Name, op.Estate.Transport)
	}
	t := plant.Target{RepoURL: op.Origin, Branch: op.Scope, Service: *service}
	var out string
	if *script {
		out, err = t.Script()
	} else {
		out, err = t.Instructions()
	}
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// shellQuote re-quotes a KEY=VALUE pair whose value needs it.
//
// The station splits the env line the way a shell would, and the shell that
// invoked THIS command has already eaten the quotes: by the time
// `HOSTS="sql01 sql02"` arrives here it is one argument with a space in it.
// Joining those with spaces produces `env: HOSTS=sql01 sql02`, which the far
// side reads as HOSTS=sql01 followed by an attempt to run `sql02`.
//
// So the quoting has to be put back. Single quotes, with the standard escape
// for an embedded one, because inside single quotes a shell interprets nothing
// at all - and this string is about to be split by one on a machine nobody can
// reach.
func shellQuote(kv string) string {
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
