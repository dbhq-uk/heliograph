package site

// Diagrams for the docs pages.
//
// WHY INLINE SVG, WRITTEN HERE
//
// They inherit the page's colours through currentColor and the CSS variables,
// so they are correct in whatever the theme becomes and there is no second
// palette to keep in step. They need no request, so they cannot be the thing
// that arrives after the text. And they are text, so they diff.
//
// WHY NOT MERMAID OR AN IMAGE
//
// Mermaid is a runtime dependency and a build step to render six pictures that
// never change. A PNG cannot be themed, goes blurry on a good screen, and says
// nothing to a reader who cannot see it.
//
// EVERY ONE CARRIES A TITLE AND role="img"
//
// A diagram that says nothing to a screen reader is decoration sold as
// explanation. The title is the sentence the picture is making, not a label:
// "a diagram of the loop" helps nobody.
//
// They are referenced from markdown as a fenced block whose language is
// `diagram`, containing only the name. That keeps the source readable and the
// markdown mirror honest, since an agent reading the .md gets the caption
// rather than four hundred bytes of path data.

// diagrams maps a name to its SVG.
//
// The CAPTION is not here. It is prose, so it lives in the markdown with the
// rest of the prose, as the body of the fence - which also means an agent
// reading the .md mirror gets the sentence the picture is making rather than a
// bare name it can do nothing with.
var diagrams = map[string]string{

	"loop": `<svg viewBox="0 0 720 200" role="img" aria-labelledby="d-loop-t" class="dg">
<title id="d-loop-t">You push a step to the transport. The station pulls it, runs it, and pushes the captured log back to the same place, where you read it.</title>
<defs><marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
<path d="M0 0 L10 5 L0 10 z" fill="currentColor"/></marker></defs>
<g class="dg-box">
<rect x="8" y="62" width="150" height="60" rx="8"/>
<rect x="285" y="62" width="150" height="60" rx="8"/>
<rect x="562" y="62" width="150" height="60" rx="8"/>
</g>
<g class="dg-label">
<text x="83" y="88">you</text><text x="83" y="106" class="dg-sub">the CLI, or an agent</text>
<text x="360" y="88">transport</text><text x="360" y="106" class="dg-sub">git, share, relay, store</text>
<text x="637" y="88">station</text><text x="637" y="106" class="dg-sub">the far side</text>
</g>
<g class="dg-line">
<path d="M162 82 L281 82" marker-end="url(#ah)"/>
<path d="M439 82 L558 82" marker-end="url(#ah)"/>
<path d="M558 108 L439 108" marker-end="url(#ah)"/>
<path d="M281 108 L162 108" marker-end="url(#ah)"/>
</g>
<g class="dg-note">
<text x="221" y="74">step</text>
<text x="498" y="74">step</text>
<text x="498" y="128">log</text>
<text x="221" y="128">log</text>
</g>
<text x="360" y="172" class="dg-foot">Nothing reaches in. Every command runs because somebody with access chose to run it.</text>
</svg>`,

	"gap": `<svg viewBox="0 0 720 210" role="img" aria-labelledby="d-gap-t" class="dg">
<title id="d-gap-t">Two logs of the same run. Without timestamps the stall is invisible. With a timestamp on every line, a three minute gap after "Refreshing state" is measurable, and it names the operation that took the time.</title>
<g class="dg-label"><text x="8" y="20" class="dg-sub">WITHOUT TIMESTAMPS</text></g>
<g class="dg-mono">
<text x="8" y="46">terraform plan</text>
<text x="8" y="66">Refreshing state...</text>
<text x="8" y="86">Plan: 3 to add</text>
</g>
<text x="8" y="112" class="dg-foot">Reads perfectly. Says nothing about where the time went.</text>

<g class="dg-label"><text x="380" y="20" class="dg-sub">WITH THEM</text></g>
<g class="dg-mono">
<text x="380" y="46">09:14:00 | terraform plan</text>
<text x="380" y="66">09:14:02 | Refreshing state...</text>
<text x="380" y="86">09:17:14 | Plan: 3 to add</text>
</g>
<g class="dg-line"><path d="M374 60 L374 82"/></g>
<g class="dg-flag"><rect x="380" y="94" width="180" height="22" rx="5"/></g>
<text x="390" y="109" class="dg-flagtext">3m12s here, and it is the answer</text>
<text x="380" y="140" class="dg-foot">The gap belongs to the line BEFORE it: that is what was running.</text>
</svg>`,

	"gates": `<svg viewBox="0 0 720 210" role="img" aria-labelledby="d-gates-t" class="dg">
<title id="d-gates-t">A request passes three checks on the station: the step must declare read-only or action, an action needs CONFIRM equals yes, and the station must have been started with allow-actions. Failing any one publishes a refusal rather than running.</title>
<defs><marker id="ah2" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="6" markerHeight="6" orient="auto">
<path d="M0 0 L10 5 L0 10 z" fill="currentColor"/></marker></defs>
<g class="dg-box"><rect x="8" y="70" width="112" height="52" rx="8"/></g>
<text x="64" y="92" class="dg-label">request</text>
<text x="64" y="110" class="dg-sub">arrives</text>
<g class="dg-line"><path d="M124 96 L166 96" marker-end="url(#ah2)"/></g>

<g class="dg-gate">
<rect x="170" y="62" width="132" height="68" rx="8"/>
<rect x="326" y="62" width="132" height="68" rx="8"/>
<rect x="482" y="62" width="132" height="68" rx="8"/>
</g>
<g class="dg-label">
<text x="236" y="88">declares a mode?</text>
<text x="392" y="88">CONFIRM=yes?</text>
<text x="548" y="88">--allow-actions?</text>
</g>
<g class="dg-sub">
<text x="236" y="110">read-only or action</text>
<text x="392" y="110">for a state change</text>
<text x="548" y="110">how it was started</text>
</g>
<g class="dg-line">
<path d="M306 96 L322 96" marker-end="url(#ah2)"/>
<path d="M462 96 L478 96" marker-end="url(#ah2)"/>
<path d="M618 96 L664 96" marker-end="url(#ah2)"/>
</g>
<text x="690" y="92" class="dg-ok">runs</text>
<g class="dg-line dg-no">
<path d="M236 134 L236 162" marker-end="url(#ah2)"/>
<path d="M392 134 L392 162" marker-end="url(#ah2)"/>
<path d="M548 134 L548 162" marker-end="url(#ah2)"/>
</g>
<text x="392" y="182" class="dg-foot">Any no: refused, and the reason is published within seconds.</text>
</svg>`,

	"transports": `<svg viewBox="0 0 720 240" role="img" aria-labelledby="d-tr-t" class="dg">
<title id="d-tr-t">Five transports sit between the control side and the station: git, file share, object store, relay and bundle. The request format, the gates and the captured log are identical whichever is used, so changing transport does not mean relearning the method.</title>
<g class="dg-box"><rect x="8" y="96" width="132" height="56" rx="8"/><rect x="580" y="96" width="132" height="56" rx="8"/></g>
<text x="74" y="120" class="dg-label">control</text><text x="74" y="138" class="dg-sub">your machine</text>
<text x="646" y="120" class="dg-label">station</text><text x="646" y="138" class="dg-sub">the far side</text>
<g class="dg-chan">
<rect x="196" y="18" width="328" height="34" rx="7"/>
<rect x="196" y="60" width="328" height="34" rx="7"/>
<rect x="196" y="102" width="328" height="34" rx="7"/>
<rect x="196" y="144" width="328" height="34" rx="7"/>
<rect x="196" y="186" width="328" height="34" rx="7"/>
</g>
<g class="dg-label">
<text x="212" y="40" text-anchor="start">git</text>
<text x="212" y="82" text-anchor="start">file share</text>
<text x="212" y="124" text-anchor="start">object store</text>
<text x="212" y="166" text-anchor="start">relay</text>
<text x="212" y="208" text-anchor="start">bundle</text>
</g>
<g class="dg-sub">
<text x="508" y="40" text-anchor="end">a host both can reach</text>
<text x="508" y="82" text-anchor="end">a mounted directory</text>
<text x="508" y="124" text-anchor="end">S3, when nothing else is</text>
<text x="508" y="166" text-anchor="end">both sides dial out over HTTPS</text>
<text x="508" y="208" text-anchor="end">a file, carried by hand</text>
</g>
<g class="dg-line">
<path d="M144 124 L192 36"/><path d="M144 124 L192 78"/>
<path d="M144 124 L192 120"/><path d="M144 124 L192 162"/><path d="M144 124 L192 204"/>
<path d="M528 36 L576 124"/><path d="M528 78 L576 124"/>
<path d="M528 120 L576 124"/><path d="M528 162 L576 124"/><path d="M528 204 L576 124"/>
</g>
</svg>`,
}

// Diagram returns the SVG for a name, and whether it exists.
func Diagram(name string) (svg string, ok bool) {
	s, ok := diagrams[name]
	return s, ok
}
