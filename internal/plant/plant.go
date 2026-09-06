// Package plant produces the instructions that put a station on a far side.
//
// Today this step is prose in a chat window: somebody types out a clone URL, a
// branch and a command, and the operator retypes them. Every retyping is a
// chance to get it wrong on a machine nobody can check.
//
// So it is generated instead, from the estate that is already configured, and
// it is the SAME text whichever way it is delivered.
package plant

import (
	"fmt"
	"strings"
)

// Target is where a station is going.
type Target struct {
	RepoURL string // what the operator will clone
	Branch  string // the task branch, if not the default
	Service bool   // install it to survive logout, rather than run in a shell
}

// Script is what the far side has to run. One command, and it is the same
// command every time afterwards.
//
// It deliberately does NOT include a credential. A token pasted into a chat
// window is a token in that chat window's history forever, and the operator
// almost always has a working credential already. Where they do not, that is a
// conversation to have on purpose rather than by default.
func (t Target) Script() (string, error) {
	if t.RepoURL == "" {
		return "", fmt.Errorf("no repository URL: the far side has nothing to clone")
	}
	if strings.ContainsAny(t.RepoURL, "\n\r;|&`$") {
		// This string is pasted into somebody's shell. It is not a place to be
		// relaxed about what can be in it.
		return "", fmt.Errorf("the repository URL contains a shell metacharacter: %q", t.RepoURL)
	}
	dir := repoDir(t.RepoURL)

	var b strings.Builder
	fmt.Fprintf(&b, "git clone %s %s\n", t.RepoURL, dir)
	fmt.Fprintf(&b, "cd %s\n", dir)
	if t.Branch != "" && t.Branch != "main" && t.Branch != "master" {
		if strings.ContainsAny(t.Branch, "\n\r;|&`$ ") {
			return "", fmt.Errorf("the branch name contains a shell metacharacter: %q", t.Branch)
		}
		fmt.Fprintf(&b, "git checkout %s\n", t.Branch)
	}
	if t.Service {
		// Survives logout. Without it the loop dies when the ssh session
		// closes, which is precisely when nobody is left to notice.
		b.WriteString("./service.sh install\n")
	} else {
		b.WriteString("./start.sh\n")
	}
	return b.String(), nil
}

// Instructions is the whole message to send to an operator: what to run, what
// it will do, and what it will not.
//
// The last part is not padding. An operator is being asked to run a stranger's
// script on a production machine, and the honest answer to "what does this do"
// is what gets it approved.
func (t Target) Instructions() (string, error) {
	script, err := t.Script()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Please run this once on the control node, then you can walk away:\n\n")
	for _, l := range strings.Split(strings.TrimRight(script, "\n"), "\n") {
		b.WriteString("    " + l + "\n")
	}
	b.WriteString("\nWhat it does\n")
	b.WriteString("  Clones a private repository and starts a loop that watches it for\n")
	b.WriteString("  requests, runs them, and pushes the log back. You do not have to\n")
	b.WriteString("  relay anything after this.\n")
	b.WriteString("\nWhat it will not do\n")
	b.WriteString("  It refuses to run as root. It runs nothing that changes state unless\n")
	b.WriteString("  you start it with --allow-actions, which this command does not.\n")
	b.WriteString("  It holds no credentials of its own, so the account you run it as is\n")
	b.WriteString("  the whole blast radius.\n")
	b.WriteString("\nTo check first, changing nothing\n")
	b.WriteString("    ./start.sh --check\n")
	b.WriteString("\nTo stop it\n")
	if t.Service {
		b.WriteString("    ./service.sh stop\n")
	} else {
		b.WriteString("    Ctrl-C, or ask us to send `stop: yes`\n")
	}
	return b.String(), nil
}

// repoDir is the directory a clone of this URL lands in, which is what the
// `cd` on the next line has to name.
func repoDir(url string) string {
	s := strings.TrimSuffix(url, "/")
	s = strings.TrimSuffix(s, ".git")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return "transport"
	}
	return s
}
