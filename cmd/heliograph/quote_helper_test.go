package main

import (
	"os/exec"
	"testing"
)

// evalAssign runs one KEY=VALUE assignment through a real bash and returns what
// V ended up as.
//
// The assignment is passed as an ARGUMENT, not interpolated into the script.
// The first version built one string, and the outer shell stripped the quoting
// before eval ever saw it - so the test reported that quoting had failed when
// what had failed was the test. `V='$HOME'` came back as /home/devops, which is
// exactly the expansion the quoting exists to prevent.
func evalAssign(t *testing.T, assignment string) string {
	t.Helper()
	out, err := exec.Command("bash", "-c", `eval "$1"; printf '%s' "$V"`, "_", assignment).Output()
	if err != nil {
		t.Fatalf("eval %q: %v", assignment, err)
	}
	return string(out)
}
