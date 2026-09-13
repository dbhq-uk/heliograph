package main

import (
	"strings"
	"testing"

	"github.com/dbhq-uk/heliograph/internal/wire"
)

// `heliograph status` answers "what is this station doing". The action mode is
// the one line on that page that answers "what is this station ALLOWED to do",
// and the whole reason it is a published field rather than a guess is that the
// wrong answer is a security misrepresentation.
//
// So the rendering is held to the same three-way rule as the parser, and each
// case is asserted on the words a reader acts on rather than on the whole line.
func TestTheActionModeRendersThreeDifferentAnswers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		actions string
		want    []string
		absent  []string
	}{{
		name:    "allowed says so, and names the other half of the gate",
		actions: "allowed",
		want:    []string{"allowed", "CONFIRM=yes"},
	}, {
		name:    "refused names the flag that would change it",
		actions: "refused",
		want:    []string{"refused", "read-only", "--allow-actions"},
	}, {
		// THE CASE THE FIELD EXISTS FOR. Every station planted before this
		// change publishes nothing, and rendering that as read-only tells a
		// reader an estate is safe on the strength of a station that never
		// said so.
		name:    "silence is reported as silence, and explicitly not as read-only",
		actions: "",
		want:    []string{"not reported", "not"},
		absent:  []string{"refused"},
	}, {
		// A newer station may publish a mode this build has not heard of. The
		// value is shown verbatim, because the reader can act on a word this
		// build cannot.
		name:    "an unrecognised mode is shown verbatim and claimed for neither side",
		actions: "supervised",
		want:    []string{"supervised", "not"},
		absent:  []string{"refused"},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := actionModeLine(wire.Status{State: "idle", Actions: tc.actions})
			if !strings.HasPrefix(got, "actions:") {
				t.Errorf("the line does not name its field: %q", got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("%q is missing from %q", w, got)
				}
			}
			for _, a := range tc.absent {
				if strings.Contains(got, a) {
					t.Errorf("%q appears in %q, and a reader would act on it", a, got)
				}
			}
		})
	}
}

// The four renderings have to differ from each other. Two cases that print the
// same sentence are two cases the reader cannot tell apart, which is the
// collapse the field was added to prevent - and it is easy to reintroduce by
// sharing a default arm.
func TestNoTwoActionModeRenderingsAreTheSame(t *testing.T) {
	seen := map[string]string{}
	for _, actions := range []string{"allowed", "refused", "", "supervised"} {
		line := actionModeLine(wire.Status{State: "idle", Actions: actions})
		if prev, dup := seen[line]; dup {
			t.Errorf("%q and %q render identically: %q", prev, actions, line)
		}
		seen[line] = actions
	}
}

// The status page aligns its values in a column. A label of a different width
// breaks the column, and the page is read by somebody scanning it.
func TestTheActionModeLineIsAlignedWithTheRest(t *testing.T) {
	// `state:    running` - the value starts at column 11.
	const col = 11
	line := actionModeLine(wire.Status{State: "idle", Actions: "allowed"})
	if i := strings.Index(line, "allowed"); i != col-1 {
		t.Errorf("the value starts at column %d, not %d: %q", i+1, col, line)
	}
}
