// Command heliograph drives a station across a gap you cannot cross yourself.
//
// This is the control side. It publishes a request, and reads the log that
// comes back. The far side is a stock heliograph-skill station and nothing
// here requires a change to it.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/estate"
	"github.com/dbhq-uk/heliograph/internal/transport"
	"github.com/dbhq-uk/heliograph/internal/wire"
)

const usage = `heliograph - run things on a machine you cannot log into

  heliograph init <estate> --dir <path>     remember a transport repo by name
  heliograph estates                        what is configured here
  heliograph send <step> [K=V ...]          publish a request, and return
  heliograph status                         what the station is doing now
  heliograph logs                           list the captured logs
  heliograph logs <name>                    print one, whole
  heliograph check                          will this work from here

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
	case "send":
		err = cmdSend(os.Args[2:])
	case "status":
		err = cmdStatus(os.Args[2:])
	case "logs":
		err = cmdLogs(os.Args[2:])
	case "check":
		err = cmdCheck(os.Args[2:])
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

func open(name string) (estate.Estate, *transport.Git, error) {
	e, err := resolve(name)
	if err != nil {
		return e, nil, err
	}
	g, err := transport.NewGit(e.Dir)
	if err != nil {
		return e, nil, err
	}
	return e, g, nil
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
	dir := fs.String("dir", "", "the working clone of the transport repo")
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

	// Attach before saving. An estate that names a directory which is not a
	// transport repo is worse than no estate at all: it fails later, from a
	// command that had every reason to expect it to work.
	g, err := transport.NewGit(abs)
	if err != nil {
		return err
	}
	e := estate.Estate{Name: name, Transport: "git", Dir: abs, Branch: g.Branch()}
	if err := e.Save(); err != nil {
		return err
	}
	fmt.Printf("estate %s -> %s on %s\n", name, abs, g.Branch())
	fmt.Printf("  %s\n", g.Describe())
	if err := g.Check(); err != nil {
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
		fmt.Printf("%-16s %s  %s\n", e.Name, e.Transport, e.Dir)
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
		env = append(env, a)
	}

	_, g, err2 := open(*name)
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
	if err := g.PutRequest(req); err != nil {
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
	_, g, err := open(*name)
	if err != nil {
		return err
	}
	s, err := g.FetchStatus()
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
	last := fs.Bool("last", false, "print the most recent log")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	_, g, err := open(*name)
	if err != nil {
		return err
	}

	if len(pos) == 0 && !*last {
		names, err := g.ListLogs()
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
		names, err := g.ListLogs()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			return fmt.Errorf("no logs yet")
		}
		target = names[0]
	}
	b, err := g.ReadLog(target)
	if err != nil {
		return err
	}
	// Whole, always. The line somebody truncates is the line they needed.
	_, err = os.Stdout.Write(b)
	return err
}

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	name := estateFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	e, g, err := open(*name)
	if err != nil {
		return err
	}
	fmt.Printf("estate:   %s\n", e.Name)
	fmt.Printf("dir:      %s\n", g.Dir())
	fmt.Printf("branch:   %s\n", g.Branch())
	fmt.Printf("%s\n", g.Describe())
	if err := g.Check(); err != nil {
		return err
	}
	fmt.Println("origin:   reachable")
	return nil
}
