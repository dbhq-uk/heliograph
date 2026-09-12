package site

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

// The compatibility matrix: every transport, every station and every control
// node, and which of them work together.
//
// THE DATA LIVES HERE RATHER THAN IN THE MARKDOWN, for the same reason the
// diagrams do. A table typed into a page is a table that disagrees with the
// next page six weeks later, and this repository has already shipped two that
// did: /transports and /hosts each carried their own partial view of the same
// facts. One source, rendered twice, cannot drift from itself.
//
// It is also testable. A pairing that names a transport or a host that does not
// exist is a build failure rather than a dead cell nobody notices.

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
	Partial  Status = "partial"  // works with a condition named in the note
	Written  Status = "written"  // code exists, nothing has ever run it
	Missing  Status = "missing"  // no implementation on this side at all
	Unproven Status = "unproven" // believed to work, never checked
)

// Transport is one channel, and it is only a transport if BOTH halves exist.
// That is why Control and Station are separate fields rather than one status:
// a transport that works on one side of the gap is not a transport.
type Transport struct {
	ID      string
	Name    string
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
	Status  Status
	Flavour string // bash, PowerShell, or both
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

// Transports, in the order a reader should meet them: the three that are
// driven end to end first, then the rest.
var Transports = []Transport{
	{ID: "git", Name: "git", Kind: Beacon, Control: Proven, Station: Proven,
		Note: "A private repository is the channel in both directions. The only transport that can bring the station a newer copy of itself.", Href: "/transports#git"},
	{ID: "relay", Name: "relay", Kind: Beacon, Control: Proven, Station: Works,
		Note: "Both sides dial out over ordinary HTTPS. Needs no git host, no storage account and no VNet - and needs heliograph-seal on the far side, which no other transport does.", Href: "/relay"},
	{ID: "share", Name: "file share", Kind: Beacon, Control: Proven, Station: Proven,
		Note: "The cheapest there is, where both machines already mount the same directory. The mount is the credential.", Href: "/transports#file-share"},
	{ID: "blob", Name: "Azure Blob", Kind: Beacon, Control: Partial, Station: Works,
		Note: "Reached through drop.sh in the station payload rather than through the heliograph binary. It is what the Azure Function App host uses.", Href: "/azure"},
	{ID: "objstore", Name: "object store", Kind: Beacon, Control: Works, Station: Missing,
		Note: "S3-compatible: AWS, R2, MinIO, B2, Spaces, Ceph. The CLI drives it and no station can read it, so the combination cannot work yet.", Href: "/transports#object-store"},
	{ID: "bundle", Name: "bundle", Kind: Beacon, Control: Works, Station: Missing,
		Note: "The only thing that makes air-gapped literally true. A person carries the file. No station can read one yet.", Href: "/air-gapped"},
	{ID: "flare", Name: "flare", Kind: Flare, Control: Works, Station: Partial,
		Note: "The one case where you CAN reach the station. The script travels with the request, so heliograph-mode stops being a control and becomes a claim the caller makes about its own file.", Href: "/flare"},
}

// Stations. Status words match /hosts exactly, because two pages disagreeing
// about whether a thing is proven is worse than either answer.
var Stations = []Station{
	{ID: "terminal", Name: "An operator's terminal", Status: Proven, Flavour: "bash, PowerShell",
		Note: "Still the best host when there is a willing person. No infrastructure request, and start.sh prints its own preflight to somebody who can read it.", Href: "/station",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes(), "flare": ok("the station must be reachable from your side")}},
	{ID: "docker", Name: "Docker", Status: Proven, Flavour: "bash",
		Note: "CI builds the image and runs a loop in it. The image plants the payload itself, so a transport with nothing to clone still has one.", Href: "/containers",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "kubernetes", Name: "Kubernetes", Status: Proven, Flavour: "bash",
		Note: "The same image, one replica. CI applies the manifest to a real cluster.", Href: "/containers",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "systemd", Name: "systemd, launchd, setsid", Status: Proven, Flavour: "bash",
		Note: "Survives a logout. A detached process inherits nothing from the shell that installed it, so the variables go in .station-env.", Href: "/service",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "blob": yes()}},
	{ID: "schtask", Name: "Windows scheduled task", Status: Proven, Flavour: "PowerShell",
		Note: "CI registers the task and reads it back. .station-env is the only way to give it a token - there is no EnvironmentFile on Windows.", Href: "/windows",
		Transports: map[string]Pairing{"git": yes(), "share": yes(), "blob": ok("through the bash station"), "relay": needs("the PowerShell relay transport is not written yet")}},
	{ID: "pipeline", Name: "GitHub Actions, Azure Pipelines", Status: Written, Flavour: "bash",
		Note: "Often the one machine in an estate that can already reach both sides. Git-only by design: the push is the trigger, and that is what keeps the latency down.", Href: "/pipelines",
		Transports: map[string]Pairing{"git": ok("the push is the trigger, which is the whole point")}},
	{ID: "azure-container", Name: "Azure ACI, Web App, Container Apps Job", Status: Proven, Flavour: "bash",
		Note: "Deployed live, then torn down. The templates take a transport through two maps rather than a parameter per transport.", Href: "/azure",
		Transports: map[string]Pairing{"git": yes(), "blob": yes(), "relay": needs("the two key files have to be put on the host"), "share": needs("the share has to be mounted")}},
	{ID: "azure-vm", Name: "Azure VM", Status: Proven, Flavour: "bash",
		Note: "Deployed live on a Standard_D2s_v3. The exception to everything else here: a bare VM has no image, so git clone is how the toolkit arrives whatever transport carries the logs.", Href: "/azure",
		Transports: map[string]Pairing{"git": yes(), "blob": yes(), "relay": needs("the key files, and a git host reachable once at first boot"), "share": needs("the share has to be mounted")}},
	{ID: "azure-function", Name: "Azure Function App", Status: Written, Flavour: "bash",
		Note: "A timer, not a loop. Validated and never deployed. It is the host the intercom was written for, because a Function App has a public endpoint while sitting inside the VNet.", Href: "/azure",
		Transports: map[string]Pairing{"blob": ok("through pigeonhole.sh"), "flare": ok("the one host with a reachable endpoint"), "git": needs("there is no git in the image")}},
	{ID: "recipe", Name: "ECS Fargate, Cloud Run, anything else", Status: Missing, Flavour: "bash",
		Note: "Recipes against the host contract, not templates. Issue #5 settled that deliberately: a template that has never started a station spends the credibility of the ones that have.", Href: "/hosts",
		Transports: map[string]Pairing{"git": unsure("against the contract, never run"), "relay": unsure("against the contract, never run"), "share": unsure("against the contract, never run"), "blob": unsure("against the contract, never run")}},
}

// Controllers. The near side had never been written down anywhere before this
// page, which is how two of these came to be true and undocumented.
var Controllers = []Controller{
	{ID: "cli", Name: "Linux, macOS or Windows", Status: Proven,
		Note: "One static binary, amd64 or arm64, no runtime. Only the git transport shells out to anything - the rest are the binary alone.", Href: "/install",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "objstore": yes(), "bundle": yes(), "flare": ok("through intercom.sh")}},
	{ID: "agent", Name: "An AI agent, over MCP", Status: Proven,
		Note: "heliograph mcp serves the same commands as typed tools over stdio. The gates do not move: a tool call publishes a request, and the station still decides whether to run it.", Href: "/mcp",
		Transports: map[string]Pairing{"git": yes(), "relay": yes(), "share": yes(), "objstore": yes(), "bundle": yes()}},
	{ID: "nocli", Name: "A machine with git and no binary", Status: Unproven,
		Note: "Already possible and documented nowhere. A request is a key: value text file, so a laptop that permits git and refuses new binaries can write it, push it, and read the log back out of the repository.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("write the request by hand, push, read the log back")}},
	{ID: "cloud-agent", Name: "Claude Code on the web", Status: Unproven,
		Note: "A cloud sandbox is a real shell, so the binary simply runs. The session proxy is what decides: private hosts and self-hosted infrastructure are refused by the allowlist, so a GitHub-hosted transport repo is the combination with a chance.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("only where the session proxy allows the host"), "relay": needs("the relay's own domain is unlikely to be on the allowlist")}},
	{ID: "phone", Name: "Android, through Termux", Status: Unproven,
		Note: "The arm64 Linux binary is already built and git is a Termux package, so this is a check rather than a port. Nobody has run it.", Href: "/roadmap",
		Transports: map[string]Pairing{"git": unsure("never checked"), "relay": unsure("never checked"), "share": unsure("never checked"), "objstore": unsure("never checked")}},
}

// --- rendering ---------------------------------------------------------------

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

// Matrix renders the interactive picker plus its three reference tables.
//
// NO-JAVASCRIPT IS THE DEFAULT STATE, not a fallback bolted on afterwards.
// Everything is visible and nothing is dimmed until a script marks a
// selection, so an agent reading the HTML - which is most of this site's
// readers - gets the whole matrix rather than an empty shell and a spinner.
func Matrix() string {
	var b strings.Builder

	b.WriteString(`<div class="mx" data-matrix>`)
	b.WriteString(`<div class="mx-head"><p class="mx-hint">Pick what you already have. Anything that cannot work with it dims, and says why.</p>`)
	b.WriteString(`<button type="button" class="mx-clear" data-mx-clear hidden>Clear</button></div>`)

	b.WriteString(`<div class="mx-grid">`)
	col(&b, "Transport", "The channel", transportCells())
	col(&b, "Station", "The far side", stationCells())
	col(&b, "Controller", "Your side", controllerCells())
	b.WriteString(`</div>`)

	b.WriteString(`<p class="mx-note" data-mx-note hidden></p>`)
	b.WriteString(`</div>`)

	b.WriteString(transportTable())
	b.WriteString(stationTable())
	b.WriteString(controllerTable())
	return b.String()
}

type cell struct {
	id, name, sub, kind string
	links               []string
}

func transportCells() []cell {
	out := make([]cell, 0, len(Transports))
	for _, t := range Transports {
		// A transport with a missing half cannot be used at all, and the
		// sub-line says which half rather than a bare "no".
		sub := statusLabel(t.Station)
		if t.Station == Missing {
			sub = "no station side"
		}
		links := []string{}
		for _, s := range Stations {
			if _, okp := s.Transports[t.ID]; okp {
				links = append(links, "s:"+s.ID)
			}
		}
		for _, c := range Controllers {
			if _, okp := c.Transports[t.ID]; okp {
				links = append(links, "c:"+c.ID)
			}
		}
		out = append(out, cell{id: "t:" + t.ID, name: t.Name, sub: string(t.Kind) + ", " + sub, kind: string(t.Kind), links: links})
	}
	return out
}

func stationCells() []cell {
	out := make([]cell, 0, len(Stations))
	for _, s := range Stations {
		links := []string{}
		for id := range s.Transports {
			links = append(links, "t:"+id)
		}
		sort.Strings(links)
		for _, c := range Controllers {
			for id := range s.Transports {
				if _, okp := c.Transports[id]; okp {
					links = append(links, "c:"+c.ID)
					break
				}
			}
		}
		out = append(out, cell{id: "s:" + s.ID, name: s.Name, sub: statusLabel(s.Status) + ", " + s.Flavour, links: links})
	}
	return out
}

func controllerCells() []cell {
	out := make([]cell, 0, len(Controllers))
	for _, c := range Controllers {
		links := []string{}
		for id := range c.Transports {
			links = append(links, "t:"+id)
		}
		sort.Strings(links)
		for _, s := range Stations {
			for id := range c.Transports {
				if _, okp := s.Transports[id]; okp {
					links = append(links, "s:"+s.ID)
					break
				}
			}
		}
		out = append(out, cell{id: "c:" + c.ID, name: c.Name, sub: statusLabel(c.Status), links: links})
	}
	return out
}

func col(b *strings.Builder, title, sub string, cells []cell) {
	fmt.Fprintf(b, `<div class="mx-col"><h3>%s<span>%s</span></h3><ul>`, esc(title), esc(sub))
	for _, c := range cells {
		kind := ""
		if c.kind != "" {
			kind = fmt.Sprintf(` data-kind="%s"`, esc(c.kind))
		}
		fmt.Fprintf(b,
			`<li><button type="button" data-mx="%s" data-mx-links="%s"%s aria-pressed="false"><span class="mx-n">%s</span><span class="mx-s">%s</span></button></li>`,
			esc(c.id), esc(strings.Join(c.links, " ")), kind, esc(c.name), esc(c.sub))
	}
	b.WriteString(`</ul></div>`)
}

func transportTable() string {
	var b strings.Builder
	b.WriteString(`<h3 id="every-transport">Every transport</h3>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Transport</th><th>Shape</th><th>Your side</th><th>The far side</th><th>What it is</th></tr></thead><tbody>`)
	for _, t := range Transports {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(t.Href), esc(t.Name), esc(string(t.Kind)), esc(statusLabel(t.Control)), esc(statusLabel(t.Station)), esc(t.Note))
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func stationTable() string {
	var b strings.Builder
	b.WriteString(`<h3 id="every-station">Every station</h3>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Station</th><th>Status</th><th>Flavour</th><th>Transports</th><th>What it is</th></tr></thead><tbody>`)
	for _, s := range Stations {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(s.Href), esc(s.Name), esc(statusLabel(s.Status)), esc(s.Flavour), esc(pairs(s.Transports)), esc(s.Note))
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func controllerTable() string {
	var b strings.Builder
	b.WriteString(`<h3 id="every-controller">Every controller</h3>`)
	b.WriteString(`<div class="tw"><table><thead><tr><th>Controller</th><th>Status</th><th>Transports</th><th>What it is</th></tr></thead><tbody>`)
	for _, c := range Controllers {
		fmt.Fprintf(&b, `<tr><td><a href="%s">%s</a></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			esc(c.Href), esc(c.Name), esc(statusLabel(c.Status)), esc(pairs(c.Transports)), esc(c.Note))
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

// pairs lists a thing's transports in the canonical Transports order, so two
// rows never disagree about the order of the same four names.
func pairs(m map[string]Pairing) string {
	var out []string
	for _, t := range Transports {
		p, okp := m[t.ID]
		if !okp {
			continue
		}
		if p.Note != "" {
			out = append(out, t.Name+" ("+p.Note+")")
			continue
		}
		out = append(out, t.Name)
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, "; ")
}

func esc(s string) string { return html.EscapeString(s) }

// MatrixCSS and MatrixJS are separate from the theme's own so that a page
// without the matrix carries neither.
const MatrixCSS = `
.mx{margin:2rem 0;border:1px solid var(--ridge);border-radius:10px;padding:1rem 1.1rem 1.2rem}
.mx-head{display:flex;gap:1rem;align-items:baseline;justify-content:space-between;flex-wrap:wrap}
.mx-hint{margin:0 0 .8rem;color:var(--ink-3);font-size:.92rem}
.mx-clear{border:1px solid var(--ridge);background:none;color:inherit;border-radius:6px;padding:.2rem .6rem;font:inherit;font-size:.85rem;cursor:pointer}
.mx-clear:hover{border-color:var(--gold)}
.mx-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:1rem}
@media(max-width:760px){.mx-grid{grid-template-columns:1fr}}
.mx-col h3{margin:0 0 .5rem;font-size:.82rem;letter-spacing:.06em;text-transform:uppercase;color:var(--ink-3);display:flex;flex-direction:column}
.mx-col h3 span{text-transform:none;letter-spacing:0;font-size:.9rem;color:var(--ink-3);opacity:.75;font-weight:400}
.mx-col ul{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:.35rem}
.mx-col button{width:100%;text-align:left;border:1px solid var(--ridge);background:none;color:inherit;font:inherit;border-radius:8px;padding:.45rem .6rem;cursor:pointer;display:flex;flex-direction:column;gap:.1rem;transition:opacity .15s,border-color .15s}
.mx-col button:hover{border-color:var(--gold)}
.mx-col button:focus-visible{outline:2px solid var(--gold);outline-offset:2px}
.mx-n{font-size:.95rem}
.mx-s{font-size:.78rem;color:var(--ink-3)}
.mx-col button[aria-pressed="true"]{border-color:var(--gold);box-shadow:inset 3px 0 0 var(--gold)}
.mx-col button[data-off]{opacity:.32}
.mx-col button[data-off] .mx-n{text-decoration:line-through}
.mx-note{margin:1rem 0 0;padding:.6rem .8rem;border-left:3px solid var(--gold);background:var(--dusk);font-size:.92rem}
.mx-col button[data-kind="flare"] .mx-n:after{content:"flare";margin-left:.5rem;font-size:.68rem;letter-spacing:.04em;text-transform:uppercase;color:var(--ink-3);border:1px solid var(--ridge);border-radius:4px;padding:0 .3rem;vertical-align:.1em}
`

const MatrixJS = `
(function(){
  var root=document.querySelector('[data-matrix]');
  if(!root)return;
  var btns=[].slice.call(root.querySelectorAll('[data-mx]'));
  var note=root.querySelector('[data-mx-note]');
  var clear=root.querySelector('[data-mx-clear]');
  var picked=null;
  function reset(){
    picked=null;
    btns.forEach(function(b){b.removeAttribute('data-off');b.setAttribute('aria-pressed','false');});
    note.hidden=true;note.textContent='';
    clear.hidden=true;
  }
  function apply(id){
    if(picked===id){reset();return;}
    picked=id;
    var src=btns.filter(function(b){return b.getAttribute('data-mx')===id;})[0];
    var links=(src.getAttribute('data-mx-links')||'').split(' ').filter(Boolean);
    var group=id.charAt(0);
    var n=0;
    btns.forEach(function(b){
      var bid=b.getAttribute('data-mx');
      b.setAttribute('aria-pressed',bid===id?'true':'false');
      if(bid===id||bid.charAt(0)===group){b.removeAttribute('data-off');return;}
      if(links.indexOf(bid)>-1){b.removeAttribute('data-off');n++;}
      else b.setAttribute('data-off','');
    });
    var name=src.querySelector('.mx-n').textContent;
    note.textContent=n?name+' works with the '+n+' option(s) still lit. Anything struck through cannot carry it, and the tables below say why.'
                      :name+' has nothing it can pair with yet. The tables below say what is missing.';
    note.hidden=false;
    clear.hidden=false;
  }
  root.addEventListener('click',function(e){
    var b=e.target.closest('[data-mx]');
    if(b){apply(b.getAttribute('data-mx'));return;}
    if(e.target.closest('[data-mx-clear]'))reset();
  });
})();
`
