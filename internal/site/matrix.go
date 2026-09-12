package site

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// The compatibility matrix: every transport, every station and every control
// node, and which of them work together.
//
// THE DATA LIVES HERE RATHER THAN IN THE MARKDOWN, for the same reason the
// diagrams do. A table typed into a page is a table that disagrees with the
// next page six weeks later, and this repository has already shipped two that
// did: /transports and /hosts each carried their own partial view of the same
// facts. One source, rendered once, cannot drift from itself.
//
// It is also testable. A pairing naming a transport that does not exist is a
// build failure rather than a dead cell nobody notices.

// Kind is the shape of a transport, and it is the distinction the whole
// product rests on. The axis is what is held, and for how long: a beacon holds
// a message, a flare is a single exchange, a beam holds the connection itself.
//
// The names come from signalling, which is what a heliograph is.
type Kind string

const (
	// Beacon is store-and-forward. Both sides dial OUT to one agreed place
	// and neither ever accepts a connection. Everything shipped is one.
	Beacon Kind = "beacon"

	// Flare is direct. You can reach the station's endpoint, so there is
	// no drop in the middle - and the gates change shape, which is the part
	// that matters rather than the latency.
	Flare Kind = "flare"

	// Beam is a live channel held open in both directions until it is torn
	// down. Designed in S4 and not yet built, so nothing carries this Kind
	// yet; it is named here because the vocabulary is one thing.
	Beam Kind = "beam"
)

// Status is how far a thing has actually got, and the words are the ones the
// rest of the site already uses. "Proven" is never claimed for something CI
// has not run.
type Status string

const (
	Proven   Status = "proven"   // CI runs it, or it has been deployed live
	Works    Status = "works"    // implemented and used, not proven in CI
	Partial  Status = "partial"  // works, with a condition named in the note
	Written  Status = "written"  // code exists, nothing has ever run it
	Missing  Status = "missing"  // no implementation on this side at all
	Unproven Status = "unproven" // believed to work, never checked
)

// Transport is one channel, and it is only a transport if BOTH halves exist.
// Control and Station are separate fields rather than one status because a
// transport that works on one side of the gap is not a transport.
type Transport struct {
	ID      string
	Name    string
	Short   string // the column head, which has a few characters to work in
	Kind    Kind
	Control Status
	Station Status
	Note    string
	Href    string
}

// Station is somewhere the loop can run.
type Station struct {
	ID      string
	Name    string
	Short   string
	Status  Status
	Flavour string
	Note    string
	Href    string
	// Transports maps a transport ID to how well it works HERE. A transport
	// absent from the map does not work on this host at all.
	Transports map[string]Pairing
}

// Controller is a near side: something that can publish a request and read a
// log back.
type Controller struct {
	ID         string
	Name       string
	Status     Status
	Note       string
	Href       string
	Transports map[string]Pairing
}

// Pairing is one cell. The note is the point of it: "no" with no reason is
// the thing a reader has to go and ask somebody about.
type Pairing struct {
	Status Status
	Note   string
}

func yes() Pairing            { return Pairing{Status: Proven} }
func ok(n string) Pairing     { return Pairing{Status: Works, Note: n} }
func needs(n string) Pairing  { return Pairing{Status: Partial, Note: n} }
func unsure(n string) Pairing { return Pairing{Status: Unproven, Note: n} }

// Transports, in the order a reader should meet them: the three the CLI drives
// end to end first, then the rest, then the one that is a different shape.
var Transports = []Transport{
	{ID: "git", Name: "git", Short: "git", Kind: Beacon, Control: Proven, Station: Proven,
		Note: "A private repository is the channel in both directions. The only transport that can bring the station a newer copy of itself.", Href: "/transports#git"},
	{ID: "relay", Name: "relay", Short: "relay", Kind: Beacon, Control: Proven, Station: Works,
		Note: "Both sides dial out over ordinary HTTPS. No git host, no storage account, no VNet. The bash station needs heliograph-seal beside it; the PowerShell one needs no binary at all - its seal is managed C# that ships as source.", Href: "/relay"},
	{ID: "share", Name: "file share", Short: "share", Kind: Beacon, Control: Proven, Station: Proven,
		Note: "The cheapest there is, where both machines already mount the same directory. The mount is the credential, and that is the whole security model.", Href: "/transports#file-share"},
	{ID: "blob", Name: "Azure Blob", Short: "blob", Kind: Beacon, Control: Partial, Station: Works,
		Note: "Reached through drop.sh in the station payload rather than the heliograph binary. A VNet-local private endpoint is often the only thing reachable.", Href: "/azure"},
	{ID: "objstore", Name: "object store", Short: "S3", Kind: Beacon, Control: Works, Station: Works,
		Note: "S3-compatible: AWS, R2, MinIO, B2, Spaces, Ceph. The station signs with SigV4 in bash, over openssl and curl - no AWS CLI and no binary.", Href: "/transports#object-store"},
	{ID: "bundle", Name: "bundle", Short: "bundle", Kind: Beacon, Control: Works, Station: Works,
		Note: "The only thing that makes air-gapped literally true: a person carries the file, and no path between the two machines is needed at all. Both halves work; a round trip takes as long as somebody takes to walk.", Href: "/air-gapped"},
	{ID: "flare", Name: "flare", Short: "flare", Kind: Flare, Control: Works, Station: Partial,
		Note: "The one case where you CAN reach the station. The script travels with the request, so heliograph-mode stops being a control and becomes a claim the caller makes about its own file.", Href: "/flare"},
}

// Stations. Status words match /hosts exactly, because two pages disagreeing
// about whether a thing is proven is worse than either answer alone.
var Stations = []Station{
	{ID: "terminal", Name: "An operator's terminal", Short: "Terminal", Status: Proven, Flavour: "bash, PowerShell",
		Note: "Still the best host when there is a willing person: no infrastructure request, and start.sh prints its own preflight to somebody who can read it.", Href: "/station",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes(), "flare": ok("only where the station's endpoint is reachable from your side")}},
	{ID: "docker", Name: "Docker", Short: "Docker", Status: Proven, Flavour: "bash",
		Note: "CI builds the image and runs a loop inside it. The image plants the payload itself, so a transport with nothing to clone still has one.", Href: "/containers",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "kubernetes", Name: "Kubernetes", Short: "Kubernetes", Status: Proven, Flavour: "bash",
		Note: "The same image, one replica. CI applies the manifest to a real cluster on every run.", Href: "/containers",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "systemd", Name: "systemd, launchd, setsid", Short: "As a service", Status: Proven, Flavour: "bash",
		Note: "Survives a logout. A detached process inherits nothing from the shell that installed it, so the transport's variables go in .station-env.", Href: "/service",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "schtask", Name: "Windows scheduled task", Short: "Windows task", Status: Proven, Flavour: "PowerShell",
		Note: "CI registers the task and reads it back. .station-env is the only way to hand it a token: there is no EnvironmentFile on Windows.", Href: "/windows",
		Transports: map[string]Pairing{"git": yes(), "share": yes(), "blob": ok("through the bash station, not the PowerShell one"), "relay": yes()}},
	{ID: "pipeline", Name: "GitHub Actions, Azure Pipelines", Short: "A pipeline", Status: Written, Flavour: "bash",
		Note: "Often the one machine in an estate that can already reach both sides. Git-only by design: the push is the trigger, and that is what keeps the latency down to however long an agent takes to start.", Href: "/pipelines",
		Transports: map[string]Pairing{"git": ok("the push is the trigger, which is the entire point")}},
	{ID: "azure-container", Name: "Azure ACI, Web App, Container Apps Job", Short: "Azure containers", Status: Proven, Flavour: "bash",
		Note: "Deployed live, then torn down. The templates take a transport through two maps rather than a parameter per transport, so a new transport does not date five templates at once.", Href: "/azure",
		Transports: map[string]Pairing{"git": yes(), "blob": yes(), "relay": needs("the two key files have to be put on the host - no template mounts a volume"), "share": needs("the share has to be mounted")}},
	{ID: "azure-vm", Name: "Azure VM", Short: "Azure VM", Status: Proven, Flavour: "bash",
		Note: "Deployed live on a Standard_D2s_v3. The exception to everything else here: a bare VM has no image, so git clone is how the toolkit arrives whatever transport then carries the logs.", Href: "/azure",
		Transports: map[string]Pairing{"git": yes(), "blob": yes(), "relay": needs("the key files, and a git host reachable once at first boot"), "share": needs("the share has to be mounted")}},
	{ID: "azure-function", Name: "Azure Function App", Short: "Function App", Status: Written, Flavour: "bash",
		Note: "A timer, not a loop. Validated and never deployed. It is the host the flare was written for, because a Function App has a public endpoint while sitting inside the VNet.", Href: "/azure",
		Transports: map[string]Pairing{"blob": ok("through pigeonhole.sh, on a timer"), "flare": ok("the one host with an endpoint you can reach"), "git": needs("there is no git in the image")}},
	{ID: "recipe", Name: "ECS Fargate, Cloud Run, anything else", Short: "Your own host", Status: Missing, Flavour: "bash",
		Note: "Recipes against the host contract, not templates. Issue #5 settled that deliberately: a template that has never started a station spends the credibility of the ones that have.", Href: "/hosts",
		Transports: map[string]Pairing{"git": unsure("meets the contract, never run"), "relay": unsure("meets the contract, never run"), "share": unsure("meets the contract, never run"), "blob": unsure("meets the contract, never run")}},
}

// Controllers. The near side had never been written down anywhere before this
// page, which is how two of these came to be true and undocumented.
var Controllers = []Controller{
	{ID: "cli", Name: "The CLI, on Linux, macOS or Windows", Status: Proven,
		Note: "One static binary, amd64 or arm64, no runtime. Only the git transport shells out to anything - the rest are the binary alone.", Href: "/install",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "objstore": yes(), "bundle": yes(), "flare": ok("through intercom.sh")}},
	{ID: "agent", Name: "An AI agent, over MCP", Status: Proven,
		Note: "heliograph mcp serves the same commands as typed tools. The gates do not move: a tool call publishes a request, and the station still decides whether to run it.", Href: "/mcp",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "objstore": yes(), "bundle": yes()}},
	{ID: "nocli", Name: "git and no binary at all", Status: Unproven,
		Note: "Already possible and documented nowhere. A request is a key: value text file, so a laptop that permits git and refuses new binaries can write it, push it, and read the log back out of the repository.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("write the request by hand, push it, read the log back")}},
	{ID: "cloud-agent", Name: "Claude Code on the web", Status: Unproven,
		Note: "A cloud sandbox is a real shell, so the binary simply runs. The session proxy decides the rest: private and self-hosted hosts are refused by the allowlist.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("only where the session proxy allows the host"), "relay": needs("the relay's own domain is unlikely to be on the allowlist")}},
	{ID: "phone", Name: "Android, through Termux", Status: Unproven,
		Note: "The arm64 Linux binary is already built and git is a Termux package, so this is a check rather than a port. Nobody has run it.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("never checked"), "relay": unsure("never checked"), "share": unsure("never checked"), "objstore": unsure("never checked")}},
}

// --- the grid ----------------------------------------------------------------

// State is what one cell of the grid says, and there are only five so that the
// legend fits in a line and the colours stay distinguishable.
type State string

const (
	CellProven State = "proven"   // run in CI, or deployed live
	CellWorks  State = "works"    // implemented, not proven here
	CellNeeds  State = "needs"    // works once you do the thing in the note
	CellUntest State = "untested" // believed to work, never checked
	CellNone   State = "none"     // cannot work, and the note says why
)

func (s State) label() string {
	switch s {
	case CellProven:
		return "Proven"
	case CellWorks:
		return "Works"
	case CellNeeds:
		return "Needs a step"
	case CellUntest:
		return "Never checked"
	default:
		return "Cannot"
	}
}

// Cell is one station-by-transport combination, resolved.
type Cell struct {
	State State  `json:"s"`
	Note  string `json:"n,omitempty"`
}

// resolve is the only place a cell's state is decided, so the grid, the count
// and the detail panel cannot disagree about the same pair.
func resolve(s Station, t Transport) Cell {
	// A transport with no station side cannot work anywhere, whatever a host
	// row happens to claim. This is checked FIRST for that reason: the object
	// store and the bundle would otherwise light up rows that can never run.
	if t.Station == Missing {
		return Cell{State: CellNone, Note: t.Name + " has no station side at all yet, so no host can read one."}
	}
	p, known := s.Transports[t.ID]
	if !known {
		return Cell{State: CellNone, Note: s.Name + " cannot carry " + t.Name + "."}
	}
	switch p.Status {
	case Proven:
		return Cell{State: CellProven, Note: p.Note}
	case Works:
		return Cell{State: CellWorks, Note: p.Note}
	case Partial:
		return Cell{State: CellNeeds, Note: p.Note}
	default:
		return Cell{State: CellUntest, Note: p.Note}
	}
}

func statusLabel(s Status) string {
	switch s {
	case Proven:
		return "proven"
	case Works:
		return "works"
	case Partial:
		return "with a condition"
	case Written:
		return "written, never run"
	case Unproven:
		return "never checked"
	default:
		return "none yet"
	}
}

// payload is what the script reads. Emitted as JSON in a script tag rather
// than as a thicket of data- attributes: the panel needs whole sentences, and
// an attribute per sentence is the same data escaped twice.
type payload struct {
	Transports  []payloadTransport  `json:"t"`
	Stations    []payloadStation    `json:"s"`
	Controllers []payloadController `json:"c"`
	Cells       map[string]Cell     `json:"cells"`
}

type payloadTransport struct {
	ID, Name, Short, Kind, Note, Href string
	Control, Station                  string
}
type payloadStation struct {
	ID, Name, Short, Status, Flavour, Note, Href string
}
type payloadController struct {
	ID, Name, Status, Note, Href string
	Transports                   map[string]string `json:"tr"`
}

// Matrix renders the grid, its controls and its reference tables.
//
// NO-JAVASCRIPT IS THE DEFAULT STATE, not a fallback bolted on afterwards.
// The grid is a real table with every cell's state already on it, so an agent
// reading the HTML - which is most of this site's readers - gets the whole
// matrix rather than an empty shell waiting for a script.
func Matrix() string {
	var b strings.Builder

	b.WriteString(`<div class="mxa" data-matrix>`)
	b.WriteString(controls())
	b.WriteString(`<div class="mxa-body">`)
	b.WriteString(grid())
	b.WriteString(panel())
	b.WriteString(`</div>`)
	b.WriteString(`<p class="mxa-empty" data-empty hidden>Nothing runs under those three filters together. That is an answer rather than a bug: loosen one and the grid comes back.</p>`)
	b.WriteString(legend())
	b.WriteString(`</div>`)
	b.WriteString(dataScript())
	b.WriteString(tables())
	return b.String()
}

func controls() string {
	var b strings.Builder
	b.WriteString(`<div class="mxa-bar">`)

	b.WriteString(`<div class="mxa-fg"><span class="mxa-fl">Shape</span><div class="mxa-chips" role="radiogroup" aria-label="Transport shape">`)
	chip(&b, "kind", "all", "Both", true)
	chip(&b, "kind", string(Beacon), "Beacon", false)
	chip(&b, "kind", string(Flare), "Flare", false)
	b.WriteString(`</div></div>`)

	b.WriteString(`<div class="mxa-fg"><span class="mxa-fl">Drive it from</span><div class="mxa-chips" role="radiogroup" aria-label="Controller">`)
	chip(&b, "ctl", "any", "Anything", true)
	for _, c := range Controllers {
		chip(&b, "ctl", c.ID, c.Name, false)
	}
	b.WriteString(`</div></div>`)

	b.WriteString(`<div class="mxa-fg"><span class="mxa-fl">Show</span><div class="mxa-chips" role="radiogroup" aria-label="What to show">`)
	chip(&b, "only", "all", "Everything", true)
	chip(&b, "only", "runs", "Only what runs", false)
	chip(&b, "only", "proven", "Only what is proven", false)
	b.WriteString(`</div></div>`)

	// The count is the readout that makes a filter feel like it did something,
	// and it is a fact rather than decoration: 70 combinations exist and most
	// of them do not work.
	fmt.Fprintf(&b, `<p class="mxa-count" data-count><b>%d</b> of %d run</p>`,
		countRunnable(), len(Stations)*len(Transports))
	b.WriteString(`</div>`)
	// The shape definitions, one line each, shown only while that shape is
	// selected. They used to be four paragraphs above the grid, which is the
	// thing this page was asked to stop being.
	b.WriteString(`<p class="mxa-def" data-def="beacon">` +
		`<b>Beacon:</b> a dead letter drop. You cannot reach the far side, it cannot reach you, and both reach one agreed place. No inbound port, ever. ` +
		`<a href="/transports">More</a></p>`)
	b.WriteString(`<p class="mxa-def" data-def="flare">` +
		`<b>Flare:</b> you can reach the station directly, so the script travels WITH the request - and the mode header stops being a gate and becomes a claim the caller makes. ` +
		`<a href="/flare">More</a></p>`)
	return b.String()
}

func countRunnable() int {
	n := 0
	for _, s := range Stations {
		for _, t := range Transports {
			if c := resolve(s, t); c.State != CellNone {
				n++
			}
		}
	}
	return n
}

// chip renders one filter option.
//
// These are RADIOS, not toggles: exactly one of each group is in force at a
// time, and aria-pressed on a set of mutually exclusive buttons tells a screen
// reader they are independent switches - so three of them read as "pressed"
// with nothing saying only one can be.
func chip(b *strings.Builder, group, val, label string, on bool) {
	checked, tab := "false", "-1"
	if on {
		checked, tab = "true", "0"
	}
	fmt.Fprintf(b, `<button type="button" class="mxa-chip" role="radio" data-f="%s" data-v="%s" aria-checked="%s" tabindex="%s">%s</button>`,
		esc(group), esc(val), checked, tab, esc(label))
}

func grid() string {
	var b strings.Builder
	b.WriteString(`<div class="mxa-gridwrap"><table class="mxa-grid"><caption class="vh">Which transport works on which station</caption>`)

	b.WriteString(`<thead><tr><td class="mxa-corner"><span>station</span><span>transport</span></td>`)
	for _, t := range Transports {
		fmt.Fprintf(&b,
			`<th scope="col" data-col="%s" data-kind="%s"><a href="%s">%s</a><em>%s</em></th>`,
			esc(t.ID), esc(string(t.Kind)), esc(t.Href), esc(t.Short), esc(string(t.Kind)[:4]))
	}
	b.WriteString(`</tr></thead><tbody>`)

	first := true
	for _, s := range Stations {
		fmt.Fprintf(&b, `<tr data-row="%s"><th scope="row"><a href="%s">%s</a><em>%s</em></th>`,
			esc(s.ID), esc(s.Href), esc(s.Short), esc(s.Flavour))
		for _, t := range Transports {
			c := resolve(s, t)
			// Roving tabindex: exactly ONE cell button is in the tab order.
			// Seventy tabbable buttons is a grid nobody reaches the far side
			// of, and a table with tabindex=0 on top of them is seventy-one.
			tab, pressed := "-1", "false"
			if first {
				tab, pressed = "0", "true"
				first = false
			}
			fmt.Fprintf(&b,
				`<td data-cell="%s" data-state="%s" data-kind="%s"><button type="button" tabindex="%s" aria-pressed="%s" aria-label="%s">`+
					`<span class="mxa-dot"></span><span class="vh">%s</span></button></td>`,
				esc(s.ID+"."+t.ID), esc(string(c.State)), esc(string(t.Kind)), tab, pressed,
				esc(s.Name+", over "+t.Name+": "+c.State.label()), esc(c.State.label()))
		}
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

// panel carries a real first cell rather than an empty box, because the empty
// box is what a reader sees for as long as it takes them to work out that the
// grid is clickable.
func panel() string {
	s, t := Stations[0], Transports[0]
	c := resolve(s, t)
	var b strings.Builder
	// aria-live sits on a hidden region that ONLY pinning writes to, not on
	// the panel. The panel also updates on hover, and a live region that
	// announces every cell a pointer crosses is a screen reader talking over
	// itself for the width of the grid.
	b.WriteString(`<p class="vh" data-say aria-live="polite"></p>`)
	b.WriteString(`<aside class="mxa-panel" data-panel>`)
	fmt.Fprintf(&b, `<p class="mxa-state" data-panel-state data-state="%s">%s</p>`, esc(string(c.State)), esc(c.State.label()))
	fmt.Fprintf(&b, `<h3 data-panel-title>%s <span>over</span> %s</h3>`, esc(s.Name), esc(t.Name))
	fmt.Fprintf(&b, `<p data-panel-note>%s</p>`, esc(firstNonEmpty(c.Note, "Proven: CI runs this combination on every push.")))
	b.WriteString(`<dl class="mxa-meta">`)
	fmt.Fprintf(&b, `<div><dt>Shape</dt><dd data-panel-kind>%s</dd></div>`, esc(string(t.Kind)))
	fmt.Fprintf(&b, `<div><dt>Station</dt><dd data-panel-flavour>%s</dd></div>`, esc(s.Flavour))
	fmt.Fprintf(&b, `<div><dt>Your side</dt><dd data-panel-control>%s</dd></div>`, esc(statusLabel(t.Control)))
	b.WriteString(`</dl>`)
	fmt.Fprintf(&b, `<p class="mxa-links"><a data-panel-tlink href="%s">About %s</a> <a data-panel-slink href="%s">About %s</a></p>`,
		esc(t.Href), esc(t.Name), esc(s.Href), esc(s.Short))
	b.WriteString(`<p class="mxa-hint">Tap a cell, or use the arrow keys.</p>`)
	b.WriteString(`<button type="button" class="mxa-close" data-close aria-label="Close details">Close</button>`)
	b.WriteString(`</aside>`)
	return b.String()
}

func legend() string {
	var b strings.Builder
	b.WriteString(`<ul class="mxa-legend">`)
	for _, s := range []State{CellProven, CellWorks, CellNeeds, CellUntest, CellNone} {
		fmt.Fprintf(&b, `<li data-state="%s"><span class="mxa-dot"></span>%s</li>`, esc(string(s)), esc(s.label()))
	}
	b.WriteString(`</ul>`)
	return b.String()
}

func dataScript() string {
	p := payload{Cells: map[string]Cell{}}
	for _, t := range Transports {
		p.Transports = append(p.Transports, payloadTransport{
			ID: t.ID, Name: t.Name, Short: t.Short, Kind: string(t.Kind), Note: t.Note, Href: t.Href,
			Control: statusLabel(t.Control), Station: statusLabel(t.Station)})
	}
	for _, s := range Stations {
		p.Stations = append(p.Stations, payloadStation{
			ID: s.ID, Name: s.Name, Short: s.Short, Status: statusLabel(s.Status),
			Flavour: s.Flavour, Note: s.Note, Href: s.Href})
		for _, t := range Transports {
			p.Cells[s.ID+"."+t.ID] = resolve(s, t)
		}
	}
	for _, c := range Controllers {
		tr := map[string]string{}
		for id, pr := range c.Transports {
			tr[id] = firstNonEmpty(pr.Note, statusLabel(pr.Status))
		}
		p.Controllers = append(p.Controllers, payloadController{
			ID: c.ID, Name: c.Name, Status: statusLabel(c.Status), Note: c.Note, Href: c.Href, Transports: tr})
	}
	j, err := json.Marshal(p)
	if err != nil {
		// Unreachable with these types, and silence here would be a grid that
		// renders and never responds.
		return `<p class="missing">the matrix data could not be built</p>`
	}
	// A "</script>" inside the JSON would close the tag early and take the
	// rest of the page with it. encoding/json escapes "<", ">" and "&" to
	// \u003c, \u003e and \u0026 by DEFAULT, which is what actually prevents
	// it - SetEscapeHTML(true) is the zero value and Marshal always applies it.
	//
	// This used to carry a ReplaceAll of "<" with "<", which is a no-op that
	// read like a guard: the test below passed on Go's escaping while
	// appearing to prove the line. Do not "restore" it. If this ever moves to
	// a json.Encoder, SetEscapeHTML(false) is the thing that would break it,
	// and the test below is what would say so.
	return `<script type="application/json" id="mx-data">` + string(j) + `</script>`
}

func tables() string {
	var b strings.Builder

	b.WriteString(`<details class="mxa-tables"><summary>Every transport, in full</summary>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Transport</th><th>Shape</th><th>Your side</th><th>The far side</th><th>What it is</th></tr></thead><tbody>`)
	for _, t := range Transports {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(t.Href), esc(t.Name), esc(string(t.Kind)), esc(statusLabel(t.Control)), esc(statusLabel(t.Station)), esc(t.Note))
	}
	b.WriteString(`</tbody></table></div></details>`)

	b.WriteString(`<details class="mxa-tables"><summary>Every station, in full</summary>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Station</th><th>Status</th><th>Flavour</th><th>What it is</th></tr></thead><tbody>`)
	for _, s := range Stations {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(s.Href), esc(s.Name), esc(statusLabel(s.Status)), esc(s.Flavour), esc(s.Note))
	}
	b.WriteString(`</tbody></table></div></details>`)

	b.WriteString(`<details class="mxa-tables"><summary>Every controller, in full</summary>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Controller</th><th>Status</th><th>Transports</th><th>What it is</th></tr></thead><tbody>`)
	for _, c := range Controllers {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(c.Href), esc(c.Name), esc(statusLabel(c.Status)), esc(pairs(c.Transports)), esc(c.Note))
	}
	b.WriteString(`</tbody></table></div></details>`)
	return b.String()
}

// pairs lists a thing's transports in the canonical Transports order, so two
// rows never disagree about the order of the same four names.
func pairs(m map[string]Pairing) string {
	var out []string
	for _, t := range Transports {
		if _, known := m[t.ID]; known {
			out = append(out, t.Name)
		}
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, ", ")
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func esc(s string) string { return html.EscapeString(s) }

// MatrixMarkdown is the grid as a markdown table, for the .md mirror.
//
// THE MIRROR IS NOT A COURTESY COPY. Agents read it far more than people do,
// and a mirror carrying the sentence "here is a grid" and no grid is a page
// that lies to its largest audience - the same failure the no-script test
// guards against one layer up, which is how this was found: matrix.html came
// out 23x its mirror, outside the 3x-17x the build asserts, because the mirror
// had none of the data in it.
func MatrixMarkdown() string {
	var b strings.Builder

	b.WriteString("| station | ")
	for _, t := range Transports {
		b.WriteString(t.Name + " | ")
	}
	b.WriteString("\n|---|")
	for range Transports {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, s := range Stations {
		b.WriteString("| " + s.Short + " | ")
		for _, t := range Transports {
			c := resolve(s, t)
			if c.State == CellNone {
				b.WriteString("no | ")
				continue
			}
			cellText := c.State.label()
			if c.Note != "" {
				cellText += " - " + c.Note
			}
			b.WriteString(cellText + " | ")
		}
		b.WriteString("\n")
	}

	b.WriteString("\n**Every transport**\n\n| transport | shape | your side | the far side | what it is |\n|---|---|---|---|---|\n")
	for _, t := range Transports {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			t.Name, t.Kind, statusLabel(t.Control), statusLabel(t.Station), t.Note)
	}

	b.WriteString("\n**Every station**\n\n| station | status | flavour | what it is |\n|---|---|---|---|\n")
	for _, s := range Stations {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", s.Name, statusLabel(s.Status), s.Flavour, s.Note)
	}

	b.WriteString("\n**Every controller**\n\n| controller | status | transports | what it is |\n|---|---|---|---|\n")
	for _, c := range Controllers {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", c.Name, statusLabel(c.Status), pairs(c.Transports), c.Note)
	}
	return b.String()
}

// ExpandMatrix replaces the ```matrix fence in a page body with the markdown
// table, so the mirror carries the data the HTML draws.
func ExpandMatrix(body string) string {
	const open = "```matrix"
	i := strings.Index(body, open)
	if i < 0 {
		return body
	}
	rest := body[i+len(open):]
	j := strings.Index(rest, "```")
	if j < 0 {
		return body
	}
	return body[:i] + MatrixMarkdown() + rest[j+3:]
}
