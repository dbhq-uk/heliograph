package plant

import (
	"strings"
	"testing"
)

func TestScriptClonesAndStarts(t *testing.T) {
	got, err := Target{RepoURL: "git@git.example:ops/payments-transport.git"}.Script()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"git clone git@git.example:ops/payments-transport.git payments-transport",
		"cd payments-transport",
		"./start.sh",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// The cd has to name the directory the clone actually lands in, or the next
// line runs in the wrong place. This is the sort of thing that is obvious in
// review and wrong in practice.
func TestTheDirectoryMatchesWhatCloneProduces(t *testing.T) {
	for _, tc := range []struct{ url, dir string }{
		{"git@git.example:ops/payments.git", "payments"},
		{"https://git.example/ops/payments.git", "payments"},
		{"https://git.example/ops/payments", "payments"},
		{"https://git.example/ops/payments/", "payments"},
		{"ssh://git@h:2222/ops/deep/nested.git", "nested"},
	} {
		got, err := Target{RepoURL: tc.url}.Script()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "cd "+tc.dir+"\n") {
			t.Errorf("%s: expected `cd %s`, got:\n%s", tc.url, tc.dir, got)
		}
	}
}

func TestABranchIsCheckedOut(t *testing.T) {
	got, err := Target{RepoURL: "https://h/x.git", Branch: "task/dns-timeouts"}.Script()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "git checkout task/dns-timeouts") {
		t.Errorf("branch not checked out:\n%s", got)
	}
}

// The default branch needs no checkout, and emitting one is noise in a message
// somebody has to read and trust.
func TestTheDefaultBranchIsNotCheckedOut(t *testing.T) {
	for _, b := range []string{"", "main", "master"} {
		got, err := Target{RepoURL: "https://h/x.git", Branch: b}.Script()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, "checkout") {
			t.Errorf("%q produced a checkout:\n%s", b, got)
		}
	}
}

func TestServiceInstallsInsteadOfRunningInAShell(t *testing.T) {
	got, err := Target{RepoURL: "https://h/x.git", Service: true}.Script()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "./service.sh install") {
		t.Errorf("no service install:\n%s", got)
	}
	if strings.Contains(got, "./start.sh\n") {
		t.Errorf("both a service install and a foreground start:\n%s", got)
	}
}

// This text is pasted into somebody's shell on a production machine. It is not
// a place to be relaxed about what can be in it.
func TestRefusesAShellMetacharacter(t *testing.T) {
	for _, bad := range []string{
		"https://h/x.git; rm -rf /",
		"https://h/x.git && curl evil",
		"https://h/$(whoami).git",
		"https://h/x.git\necho pwned",
		"https://h/`id`.git",
	} {
		if _, err := (Target{RepoURL: bad}).Script(); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, err := (Target{RepoURL: "https://h/x.git", Branch: "a; rm -rf /"}).Script(); err == nil {
		t.Error("accepted a branch with a metacharacter")
	}
}

func TestRefusesAnEmptyURL(t *testing.T) {
	if _, err := (Target{}).Script(); err == nil {
		t.Error("accepted an empty repository URL")
	}
}

// An operator is being asked to run a stranger's script on a production
// machine. The honest answer to "what does this actually do" is what gets it
// approved, so it is part of the output rather than something to be asked for.
func TestInstructionsSayWhatItWillNotDo(t *testing.T) {
	got, err := Target{RepoURL: "https://h/x.git"}.Instructions()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"refuses to run as root",
		"--allow-actions",
		"blast radius",
		"./start.sh --check",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("instructions do not mention %q:\n%s", want, got)
		}
	}
}

// A credential in this text is a credential in a chat log forever.
func TestInstructionsCarryNoCredential(t *testing.T) {
	got, err := Target{RepoURL: "https://h/x.git"}.Instructions()
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"GIT_TOKEN", "password", "ghp_", "glpat-", "Authorization"} {
		if strings.Contains(got, bad) {
			t.Errorf("instructions mention %q:\n%s", bad, got)
		}
	}
}
