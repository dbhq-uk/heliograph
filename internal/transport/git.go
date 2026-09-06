package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// Git carries the documents in a git repository, which is the transport the
// skill has always used and the one an estate is most likely to already permit.
type Git struct {
	dir    string // a working clone of the transport repo
	branch string
}

const (
	requestPath = "station/request"
	statusPath  = "station/status"
	logDir      = "ops-logs"

	// Long enough for a slow link, short enough that a wrong remote is
	// reported rather than waited on. A hang here is indistinguishable from a
	// slow network, and the reader usually cannot ask anybody which it is.
	gitTimeout = 60 * time.Second
)

// NewGit attaches to an existing working clone.
//
// It does not clone. Deciding where a checkout lives, and with what
// credential, is the caller's business and is worth being explicit about.
func NewGit(dir string) (*Git, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	g := &Git{dir: abs}
	if _, err := g.git("rev-parse", "--git-dir"); err != nil {
		return nil, fmt.Errorf("%s is not a git repository: %w", abs, err)
	}
	b, err := g.git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	g.branch = strings.TrimSpace(b)
	if g.branch == "" || g.branch == "HEAD" {
		return nil, fmt.Errorf("%s is not on a branch: check out the task branch first", abs)
	}
	return g, nil
}

// Branch is the branch this transport is bound to.
func (g *Git) Branch() string { return g.branch }

// Dir is the working clone.
func (g *Git) Dir() string { return g.dir }

func (g *Git) git(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.dir
	// Never prompt. A credential prompt from a CLI that may be running
	// unattended hangs forever and says nothing about why.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat")

	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(gitTimeout):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return "", fmt.Errorf("git %s timed out after %s", args[0], gitTimeout)
	}
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// FetchStatus reads the station's published status from the remote ref.
//
// From the remote rather than the working tree, because the working tree is
// only as fresh as the last pull and a stale status reads exactly like a
// station that has stopped responding.
func (g *Git) FetchStatus() (wire.Status, error) {
	if _, err := g.git("fetch", "--quiet", "origin", g.branch); err != nil {
		return wire.Status{}, err
	}
	out, err := g.git("show", "origin/"+g.branch+":"+statusPath)
	if err != nil {
		// A station that has never run has published no status. Not a fault,
		// and the caller wants to say so plainly rather than raise it.
		return wire.Status{}, nil
	}
	return wire.ParseStatus([]byte(out))
}

// PutRequest publishes a request and does not return until it has arrived.
//
// It rebases rather than merges or forces. The station pushes far more often
// than the control side does - a status commit on every transition, a progress
// snapshot every 60 seconds, and the log - so the remote will routinely have
// moved while the next request was being written. A force push here would
// destroy the very evidence the loop exists to deliver.
func (g *Git) PutRequest(r wire.Request) error {
	if err := r.Validate(); err != nil {
		return err
	}
	full := filepath.Join(g.dir, filepath.FromSlash(requestPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, r.Marshal(), 0o644); err != nil {
		return err
	}
	if _, err := g.git("add", "--", requestPath); err != nil {
		return err
	}
	// Nothing staged means nothing changed, which means the id did not change,
	// which means no run would start. Say so rather than pushing an empty
	// commit and reporting success.
	if _, err := g.git("diff", "--cached", "--quiet", "--", requestPath); err == nil {
		return errors.New("the request is unchanged, so no run would start: change the id")
	}
	if _, err := g.git("commit", "--quiet", "-m",
		"request: "+r.ID+" ***NO_CI***", "--", requestPath); err != nil {
		return err
	}

	// Rebase, then push. One retry: the window between the rebase and the push
	// is small but the station writes into it often enough to matter.
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := g.git("pull", "--rebase", "--quiet", "origin", g.branch); err != nil {
			// Never resolve a conflict and never discard the other writer's
			// work. Abort, report, and let a human decide.
			_, _ = g.git("rebase", "--abort")
			return fmt.Errorf("could not rebase onto origin/%s: %w", g.branch, err)
		}
		if _, err := g.git("push", "--quiet", "origin", "HEAD:"+g.branch); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("the request is committed locally but did not reach the remote: %w", lastErr)
}

// ListLogs names the captured logs on the remote branch, newest first.
func (g *Git) ListLogs() ([]string, error) {
	if _, err := g.git("fetch", "--quiet", "origin", g.branch); err != nil {
		return nil, err
	}
	out, err := g.git("ls-tree", "--name-only", "origin/"+g.branch+":"+logDir)
	if err != nil {
		return nil, nil // no logs yet
	}
	var names []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasSuffix(l, ".txt") {
			names = append(names, l)
		}
	}
	// The names carry a UTC stamp, so a reverse lexical sort is a reverse
	// chronological one. The log you want is almost always the last run.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

// ReadLog returns one log whole.
//
// No line limit, ever. The line somebody truncates is the line they needed.
func (g *Git) ReadLog(name string) ([]byte, error) {
	clean, err := safeLogName(name)
	if err != nil {
		return nil, err
	}
	if _, err := g.git("fetch", "--quiet", "origin", g.branch); err != nil {
		return nil, err
	}
	out, err := g.git("show", "origin/"+g.branch+":"+logDir+"/"+clean)
	if err != nil {
		return nil, fmt.Errorf("no log named %q on origin/%s", clean, g.branch)
	}
	return []byte(out), nil
}

// safeLogName refuses anything that escapes the log directory.
//
// The name reaches a `git show` path and, for a caller that saves it, a
// filesystem path. `../../.ssh/id_ed25519` must not be a log name.
func safeLogName(name string) (string, error) {
	if name == "" {
		return "", errors.New("no log name given")
	}
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("a log name may not be an absolute path: %q", name)
	}
	name = strings.TrimPrefix(name, logDir+"/")
	clean := path.Clean(name)
	if clean != name || strings.Contains(clean, "/") || clean == ".." || clean == "." {
		return "", fmt.Errorf("a log name may not contain a path: %q", name)
	}
	return clean, nil
}

// Check answers "will this work from here" and changes nothing.
//
// Read access is not write access, and learning the difference after an
// hour-long run has captured evidence it cannot deliver costs a whole round
// trip through somebody who cannot debug the machine.
func (g *Git) Check() error {
	// Against the branch this transport is bound to, NOT against the remote's
	// HEAD. A transport repo's default branch is routinely not the task branch
	// - branch-per-investigation is how the whole method works - and a bare
	// remote whose HEAD points at a branch nobody uses is ordinary. Checking
	// HEAD there reports "cannot reach origin" for a remote that is reachable
	// and correct, which sends the reader to the network for a problem that is
	// not there.
	if _, err := g.git("ls-remote", "--exit-code", "origin",
		"refs/heads/"+g.branch); err != nil {
		return fmt.Errorf("cannot reach origin/%s: %w", g.branch, err)
	}
	return nil
}

// Describe names the credential mechanism, never the value.
//
// Lengths, not secrets. A log or a screenshot of this output is routine, and
// the value is never the question.
func (g *Git) Describe() string {
	url, _ := g.git("remote", "get-url", "origin")
	url = strings.TrimSpace(url)

	scheme := "ssh or local path"
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		scheme = "https"
	}
	cred := "none configured (ssh key, agent, or a public remote)"
	if v := os.Getenv("GIT_AUTH_HEADER"); v != "" {
		cred = fmt.Sprintf("env:GIT_AUTH_HEADER (%d chars)", len(v))
	} else if v := os.Getenv("GIT_TOKEN"); v != "" {
		cred = fmt.Sprintf("env:GIT_TOKEN (%d chars)", len(v))
	} else if f := os.Getenv("GIT_TOKEN_FILE"); f != "" {
		cred = "file:" + f
	}
	return fmt.Sprintf("git %s on %s, credential: %s", scheme, g.branch, cred)
}

// compile-time proof that Git satisfies the interface the relay will also
// have to satisfy.
var _ Transport = (*Git)(nil)
