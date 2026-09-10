package site

// The visual world: an optical instrument.
//
// heliograph signals by flashing sunlight off a mirror across a valley, and the
// site is built from that rather than from the developer-tool default. Near
// black for the valley at dusk, brass for the instrument, and one hot
// white-gold for the light itself - the only pure bright on the page, spent
// where the eye should go and nowhere else.
//
// Dark is chosen from the use scene, not the category. This is read by somebody
// mid-incident, at night, beside a terminal.
const CSS = `
/* ---------------------------------------------------------------- fonts */
/* Self-hosted. A third-party font request on a docs site is a dependency
   nobody asked for, and Instrument is the right voice twice over: a display
   serif with real optical contrast, and a sans from the same drawing. */
@font-face{font-family:'Archivo';src:url('/assets/fonts/Archivo.woff2')format('woff2');
  font-weight:400 700;font-style:normal;font-display:swap}
@font-face{font-family:'JetBrains Mono';src:url('/assets/fonts/JetBrainsMono.woff2')format('woff2');
  font-weight:400 700;font-style:normal;font-display:swap}

/* --------------------------------------------------------------- tokens */
:root{
  /* Denim. A worn, slightly desaturated blue rather than a screen blue, on a
     blue-black valley - and every accent drawn from the 300-400 lightness band
     with saturation pulled back, because a full-strength blue BURNS on a dark
     surface and loses the scarcity that makes an accent work at all.

     Every pair below is measured against --night, not eyeballed:
       ink    16.6:1   ink-2  9.2:1   ink-3  5.7:1
       denim   7.8:1   signal 11.7:1  flash 17.1:1
       dark text on the denim button: 7.4:1
     The commonest failure in dev tooling is a slate-500 secondary at 4.0:1.
     --ink-3 is the floor here and it is 5.7:1. */
  --night:#080C12;          /* the valley */
  --dusk:#0D141C;           /* raised surfaces */
  --ridge:#18222E;          /* borders, edges */
  --slate:#243244;

  --brass:#4E7FB3;          /* the instrument, in shade */
  --gold:#7BA7D4;           /* the signal */
  --flash:#E6F1FB;          /* the light itself. The only near-white on the page. */
  --signal:#A8CCEC;

  --ink:#E7EDF4;
  --ink-2:#A3B4C6;
  --ink-3:#7A8CA0;          /* 5.2:1, captions only */

  --measure:68ch;
  --gutter:clamp(1rem,2.2vw,1.8rem);
  --ease:cubic-bezier(.16,1,.3,1);
}

/* ----------------------------------------------------------------- base */
*,*::before,*::after{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{
  margin:0;background:var(--night);color:var(--ink);
  font:400 17px/1.65 'Archivo',ui-sans-serif,system-ui,sans-serif;
  font-feature-settings:'kern' 1;
  -webkit-font-smoothing:antialiased;
  overflow-x:hidden;
}

/* The browser's own surfaces belong to the design too. These ship with
   defaults that belong to no design system, and theming them is the cheapest
   signal that a page was built rather than assembled. */
::selection{background:var(--gold);color:var(--night)}
::-webkit-scrollbar{width:11px;height:11px}
::-webkit-scrollbar-track{background:var(--night)}
::-webkit-scrollbar-thumb{background:var(--slate);border-radius:6px;border:3px solid var(--night)}
::-webkit-scrollbar-thumb:hover{background:var(--brass)}
html{scrollbar-color:var(--slate) var(--night);scrollbar-width:thin}
:focus-visible{outline:2px solid var(--gold);outline-offset:3px;border-radius:2px}
a{color:var(--gold);text-decoration-color:color-mix(in srgb,var(--gold) 42%,transparent);
  text-underline-offset:.22em;text-decoration-thickness:1px;
  transition:text-decoration-color .18s var(--ease),color .18s var(--ease)}
a:hover{color:var(--flash);text-decoration-color:var(--flash)}

/* --------------------------------------------------------------- header */
.site-header,.mobile-bar{
  position:sticky;top:0;z-index:50;
  display:flex;gap:2rem;align-items:center;flex-wrap:wrap;
  /* Full-bleed background and border, content aligned to the shell. Without
     the max() the brand sits a few pixels off the sidebar below it, which is
     the kind of misalignment nobody names and everybody sees. */
  --gutter:clamp(1rem,2.2vw,1.8rem);
  --edge:max(var(--gutter),calc((100vw - 90rem)/2 + var(--gutter)));
  padding:.85rem var(--edge);
  background:color-mix(in srgb,var(--night) 82%,transparent);
  backdrop-filter:blur(14px) saturate(1.3);
  border-bottom:1px solid var(--ridge);
}
.brand{display:flex;align-items:center;gap:.6rem;text-decoration:none;color:var(--ink);
  font-family:'Archivo',sans-serif;font-weight:600;font-size:1.12rem;letter-spacing:-.02em}
.brand:hover{color:var(--ink)}
.brand svg{width:26px;height:26px;color:var(--gold);flex:none}
/* SCOPED to the header's own nav with a child combinator, not to every nav on
   the page. The unscoped rules leaked into .side nav and .rail nav, which then
   had to undo them one at a time. The header carries three links on the home
   page and none anywhere else - the sidebar is the navigation once you are in
   the docs. */
.site-header>nav{display:flex;gap:1.35rem;flex-wrap:nowrap;margin-left:auto}
.site-header>nav a{color:var(--ink-2);text-decoration:none;font-size:.93rem;
  position:relative;padding:.15rem 0;white-space:nowrap}
.site-header>nav a::after{content:'';position:absolute;left:0;right:100%;bottom:-2px;height:1px;
  background:var(--gold);transition:right .3s var(--ease)}
.site-header>nav a:hover{color:var(--ink)}
.site-header>nav a:hover::after{right:0}
/* Below this the three links become a deliberate second row rather than
   wrapping mid-list. A link never breaks across lines. */
@media(max-width:420px){
  .site-header{flex-wrap:wrap;gap:.55rem}
  .site-header>nav{width:100%;margin-left:0;justify-content:space-between}
}

/* ----------------------------------------------------------------- hero */
/* The authored moment. One beam crosses the valley, lands, and the log comes
   back. It runs once on load and on a slow loop after, so a page that is being
   read is not competing with its own header. */
.hero{position:relative;overflow:hidden;border-bottom:1px solid var(--ridge);min-height:min(86vh,780px);display:flex;align-items:center}
.hero canvas{position:absolute;inset:0;width:100%;height:100%;display:block}
.hero-inner{
  position:relative;z-index:2;
  max-width:min(76rem,92vw);margin:0 auto;
  width:100%;
  padding:clamp(3rem,9vh,5.5rem) clamp(1.1rem,4vw,2.5rem) clamp(9rem,22vh,13rem);
}
.hero h1{
  font-family:'Archivo',sans-serif;font-weight:600;
  font-size:clamp(2.6rem,6.6vw,4.9rem);line-height:1.02;letter-spacing:-.035em;
  margin:0 0 1.15rem;max-width:17ch;text-wrap:balance;
}
.hero h1 em{font-style:normal;color:var(--gold)}
.hero .lede{
  font-size:clamp(1.08rem,1.9vw,1.34rem);line-height:1.55;color:var(--ink-2);
  max-width:46ch;margin:0 0 2.4rem;text-wrap:pretty;
}
.cta{display:flex;gap:.85rem;flex-wrap:wrap;align-items:center}
.btn{
  display:inline-flex;align-items:center;gap:.5rem;
  padding:.7rem 1.3rem;border-radius:7px;text-decoration:none;
  font-size:.97rem;font-weight:500;letter-spacing:.005em;
  transition:transform .2s var(--ease),box-shadow .2s var(--ease),background .2s var(--ease);
}
.btn-primary{background:var(--gold);color:#08131F;
  box-shadow:0 1px 2px rgba(0,0,0,.5),0 10px 26px -12px rgba(123,167,212,.5)}
.btn-primary:hover{background:var(--signal);color:#08131F;transform:translateY(-1px);
  box-shadow:0 2px 4px rgba(0,0,0,.5),0 16px 34px -12px rgba(123,167,212,.7)}
.btn-ghost{color:var(--ink);border:1px solid var(--slate);background:color-mix(in srgb,var(--dusk) 70%,transparent)}
.btn-ghost:hover{color:var(--flash);border-color:var(--brass);transform:translateY(-1px)}

/* ------------------------------------------------------------ the strip */
/* A real captured log, with a real gap in the timestamp column. The single
   most distinctive fact about this product, shown rather than described. */
.strip{border-bottom:1px solid var(--ridge);background:var(--dusk)}
.strip-inner{max-width:min(76rem,92vw);margin:0 auto;padding:2.6rem clamp(1.1rem,4vw,2.5rem)}
.strip h2{font-size:.78rem;letter-spacing:.14em;text-transform:uppercase;color:var(--ink-3);
  font-weight:500;margin:0 0 1.1rem}
.log{font-family:'JetBrains Mono',ui-monospace,monospace;font-size:.845rem;line-height:1.95;
  font-variant-numeric:tabular-nums;overflow-x:auto;margin:0}
.log .t{color:var(--ink-3)}
.log .gap{color:var(--gold);background:linear-gradient(90deg,
  color-mix(in srgb,var(--gold) 13%,transparent),transparent 62%);
  display:block;padding-left:.4rem;margin-left:-.4rem;border-radius:3px}

/* ---------------------------------------------------------------- shell */
/* Three columns on a docs page: navigation, the reading column, and the
   page's own headings.

   The layout this replaces was a full-bleed header above a centred 68ch
   column. The logo sat hard left, the first word of the body began a third of
   the way across, and nothing shared an edge with anything - which reads as
   broken even to a reader who could not say why, and wastes most of a wide
   window doing it.

   The gutters are deliberately small. Width here is not decoration: it is how
   much of a captured log fits on one line before it wraps, and a wrapped log
   line is harder to scan for the gap that matters. */
/* ------------------------------------------------------------- diagrams */
/* Coloured from the same variables as the prose, so there is no second
   palette to keep in step and they are right in whatever the theme becomes. */
.dgw{margin:2.4rem 0;padding:0}
.dg{width:100%;height:auto;display:block;color:var(--ink-2)}
.dgw figcaption{
  margin-top:.9rem;font-size:.88rem;color:var(--ink-2);line-height:1.5;
  text-wrap:pretty;
}
.dg-box rect{fill:var(--dusk);stroke:var(--ridge);stroke-width:1}
.dg-gate rect{fill:none;stroke:var(--ridge);stroke-width:1;stroke-dasharray:3 3}
.dg-chan rect{fill:var(--dusk);stroke:var(--ridge);stroke-width:1}
.dg-line path{stroke:var(--ridge);stroke-width:1.25;fill:none}
.dg-line.dg-no path{stroke-dasharray:3 3}
.dg text{font-family:'Instrument Sans',system-ui,sans-serif}
.dg-label,.dg text.dg-label{fill:var(--ink);font-size:13px;font-weight:500;text-anchor:middle}
.dg-sub,.dg text.dg-sub{fill:var(--ink-2);font-size:11px;text-anchor:middle;letter-spacing:.02em}
.dg-note text{fill:var(--ink-2);font-size:11px;text-anchor:middle}
.dg-foot{fill:var(--ink-2);font-size:11.5px;text-anchor:middle}
.dg-mono text,.dg .dg-mono text{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;
  font-size:12.5px;fill:var(--ink-2);text-anchor:start}
.dg-ok{fill:var(--gold);font-size:13px;font-weight:500;text-anchor:middle}
.dg-flag rect{fill:color-mix(in srgb,var(--gold) 14%,transparent);stroke:var(--gold);stroke-width:1}
.dg-flagtext{fill:var(--ink);font-size:11.5px;text-anchor:start}
.missing{color:#e06c75;font-weight:600}

/* A diagram is scanned, so it may use the whole column rather than the
   reading measure. Below that it is smaller than its own labels. */
.docs-shell main .dgw{max-width:none;width:100%}
@media(max-width:560px){.dg text{font-size:14px}.dg-sub,.dg-foot{font-size:12px}}

/* ----------------------------------------------------------------- main */
/* Inside the shell the column already sets the width, so main only sets the
   measure it will not exceed. --measure is the line length prose is readable
   at; the column is wider than that so tables and code can use the room. */
main{max-width:var(--measure);margin:0;padding:2.6rem 0 6rem}
main.wide{max-width:min(76rem,92vw);margin:0 auto;padding:3.4rem clamp(1.1rem,4vw,2.5rem) 6rem}

/* A table or a code block may use the whole column: they are scanned, not
   read, and a 68ch table wraps cells that were meant to line up. */
.docs-shell main table,.docs-shell main pre{max-width:none;width:100%}

/* And a table WIDER than the column scrolls rather than being cut off. The
   host-by-transport matrix is five columns and cannot shrink to a phone; with
   body overflow-x hidden, the columns on the right simply vanished, which is
   the worst possible failure for a table whose whole content is the right-hand
   columns. A wrapper rather than display:block on the table itself, so the
   table keeps its own layout. */
.tw{overflow-x:auto;max-width:100%}
.tw table{min-width:32rem}
@media(min-width:64rem){.tw table{min-width:0}}

/* ------------------------------------------------- the documentation shell */
/* Mobile first, and that is not a style preference. The old rules built three
   columns and then took them apart twice; below 820px the sidebar became a
   wrapped row of every page with the group headings hidden, which is not
   navigation - it is a sitemap dumped above the article. Position in that row
   changed with the viewport, so there was nothing to remember, and a keyboard
   user tabbed through twenty-three links to reach the first word. */

.skip-link{
  position:fixed;top:.6rem;left:.6rem;z-index:200;
  padding:.65rem .9rem;border-radius:6px;
  background:var(--flash);color:var(--night);
  font-weight:600;text-decoration:none;
  transform:translateY(calc(-100% - 1.5rem))
}
.skip-link:focus{transform:none}
main:focus{outline:none}

.mobile-bar{position:sticky;top:0;z-index:50;justify-content:space-between}
.menu-button,.menu-close{
  display:inline-flex;align-items:center;justify-content:center;gap:.45rem;
  min-width:2.75rem;min-height:2.75rem;padding:.45rem .65rem;
  border:1px solid var(--slate,var(--ridge));border-radius:7px;
  background:var(--dusk);color:var(--ink);
  font:600 .9rem/1 'Archivo',ui-sans-serif,system-ui,sans-serif;cursor:pointer
}
.menu-button:hover,.menu-close:hover{border-color:var(--gold);color:var(--flash)}
.menu-button svg,.menu-close svg{
  width:1.25rem;height:1.25rem;fill:none;stroke:currentColor;
  stroke-width:1.8;stroke-linecap:round
}
/* Never offer a control that cannot work. */
.no-js .menu-button{display:none}

.nav-dialog{
  position:fixed;inset:0;
  width:100vw;max-width:none;height:100dvh;max-height:none;
  margin:0;padding:0;border:0;background:transparent;color:var(--ink);overflow:hidden
}
.nav-dialog::backdrop{background:rgba(4,7,11,.72)}
.nav-dialog-panel{
  width:min(20rem,calc(100vw - 2rem));height:100dvh;
  overflow-y:auto;scrollbar-gutter:stable;
  background:var(--night);border-right:1px solid var(--ridge);
  box-shadow:1rem 0 3rem rgba(0,0,0,.45)
}
.nav-dialog-head{
  position:sticky;top:0;z-index:1;
  display:flex;align-items:center;justify-content:space-between;
  min-height:3.5rem;padding:.6rem .8rem .6rem 1rem;
  background:var(--night);border-bottom:1px solid var(--ridge)
}
.nav-dialog-head h2{
  margin:0;padding:0;border:0;
  font:600 1rem/1.2 'Archivo',ui-sans-serif,system-ui,sans-serif;letter-spacing:0
}
.nav-dialog .side-nav{padding:1rem .75rem 2rem}
/* A browser with no native dialog has no UA rule hiding a closed one, so
   without this the whole drawer paints over the page for ever - and the
   no-JS sidebar appears underneath it, giving two copies of the navigation.
   Belt and braces, because the failure is total. */
.no-js .nav-dialog{display:none}
.nav-dialog:not([open]){display:none}
html:has(.nav-dialog[open]),body:has(.nav-dialog[open]){overflow:hidden}

.docs-shell{display:block;width:100%;max-width:90rem;margin:0 auto}

/* Below the desktop breakpoint the sidebar is the no-JS fallback: a grouped,
   vertical list above the article. Plainer than the drawer and still readable,
   which is the right way for a fallback to differ. */
.side{position:static;padding:1rem;border-bottom:1px solid var(--ridge)}
.js .side{display:none}
.side-brand{display:none}

.side-nav{margin:0}
.side-group+.side-group{margin-top:1.35rem}
.side-nav .grp{
  margin:0 0 .4rem;padding:0 .7rem;color:var(--ink-3,var(--ink-2));
  font-size:.72rem;font-weight:600;letter-spacing:.12em;text-transform:uppercase
}
.side-nav ul{margin:0;padding:0;list-style:none}
.side-nav li{margin:0}
.side-nav a{
  display:flex;align-items:center;min-height:2.75rem;padding:.45rem .7rem;
  border-radius:6px;color:var(--ink-2);text-decoration:none;
  font-size:.92rem;line-height:1.35;
  transition:color .15s var(--ease),background .15s var(--ease)
}
.side-nav a::after{display:none}
.side-nav a:hover{color:var(--ink);background:var(--dusk)}
.side-nav a.here{color:var(--flash);background:var(--dusk);font-weight:600;
  box-shadow:inset 2px 0 var(--gold)}

.col{min-width:0;padding:0 1.1rem}
.docs-shell main{max-width:80ch;margin:0;padding:2rem 0 5rem}
.rail{display:none}

/* Without this a heading jumped to from the rail lands under the sticky bar. */
main :where(h1,h2,h3,h4){scroll-margin-top:4.75rem}

/* 64rem: room for a permanent sidebar beside a readable column. */
@media(min-width:64rem){
  .mobile-bar{display:none}
  .docs-shell{display:grid;grid-template-columns:15rem minmax(0,1fr);align-items:start}
  .side,.js .side{
    display:block;position:sticky;top:0;align-self:start;
    height:100dvh;max-height:none;padding:0 0 2rem;
    overflow-y:auto;scrollbar-gutter:stable;
    border-right:1px solid var(--ridge);border-bottom:0
  }
  .side-brand{display:flex;min-height:3.5rem;margin:0;padding:.6rem 1rem;
    border-bottom:1px solid var(--ridge)}
  .side .side-nav{padding:1.25rem .75rem 2rem}
  .side-nav a{min-height:2.1rem;padding:.35rem .7rem}
  .col{padding:0 3rem}
  .docs-shell main{padding-top:2.75rem}
  main :where(h1,h2,h3,h4){scroll-margin-top:2rem}
}

/* 80rem: and now the rail as well. Contiguous tracks, no grid gap - the old
   layout spent two explicit gaps plus per-link margins, so a one-pixel resize
   at the old 1180 breakpoint took about 300px off the article at once. */
@media(min-width:80rem){
  .docs-shell{grid-template-columns:15rem minmax(0,1fr) 15rem}
  .rail{
    display:block;position:sticky;top:0;align-self:start;
    height:100dvh;padding:2.9rem 1rem 2rem;
    overflow-y:auto;scrollbar-gutter:stable
  }
  .rail .grp{
    margin:0 0 .6rem;color:var(--ink-2);
    font-size:.72rem;font-weight:600;letter-spacing:.12em;text-transform:uppercase
  }
  .rail nav{display:flex;flex-direction:column;gap:.05rem;margin:0}
  .rail nav a{
    padding:.3rem 0 .3rem .7rem;border-left:1px solid var(--ridge);
    color:var(--ink-2);text-decoration:none;font-size:.85rem;line-height:1.4
  }
  .rail nav a::after{display:none}
  .rail nav a:hover{color:var(--ink);border-left-color:var(--gold)}
}

main ol{margin:1.1rem 0;padding-left:1.35rem}
main ol li{margin:.35rem 0;padding-left:.25rem}
main ol li::marker{color:var(--ink-2)}

h1,h2,h3,h4{text-wrap:balance}

/* --------------------------------------------------------------- printing */
/* A page box is narrower than the desktop breakpoint, so without this every
   printed page carried the mobile bar and a Menu button that does nothing on
   paper, while the sidebar stayed hidden. Print the article and nothing else. */
@media print{
  .mobile-bar,.nav-dialog,.side,.rail,.skip-link,.site-header,footer{display:none}
  .docs-shell{display:block;max-width:none}
  .col{padding:0}
  .docs-shell main{max-width:none;padding:0}
  a[href^="/"]::after{content:" (" attr(href) ")";font-size:.85em;color:#555}
}
main h1{font-family:'Archivo',sans-serif;font-weight:600;
  font-size:clamp(2.1rem,4.4vw,3rem);line-height:1.08;letter-spacing:-.032em;margin:.2em 0 .5em}
main h2{font-family:'Archivo',sans-serif;font-weight:600;
  font-size:clamp(1.4rem,2.6vw,1.8rem);line-height:1.22;letter-spacing:-.022em;
  margin:2.6em 0 .6em;padding-top:1.4rem;border-top:1px solid var(--ridge)}
main h3{font-size:1.1rem;font-weight:600;margin:2.1em 0 .45em;letter-spacing:-.005em}
main h4{font-size:.98rem;font-weight:600;color:var(--ink-2);margin:1.7em 0 .35em}
p,li{margin:.85em 0;text-wrap:pretty}
strong{font-weight:600;color:var(--flash)}
em{color:var(--ink)}
ul{padding-left:1.15rem}
li::marker{color:var(--brass)}

code{font-family:'JetBrains Mono',ui-monospace,monospace;font-size:.88em;
  background:var(--dusk);border:1px solid var(--ridge);padding:.1em .38em;border-radius:4px;
  color:var(--gold)}
pre{background:var(--dusk);border:1px solid var(--ridge);border-radius:9px;
  padding:1.05rem 1.25rem;overflow-x:auto;
  box-shadow:0 1px 2px rgba(0,0,0,.4),0 14px 34px -22px rgba(0,0,0,.9)}
pre code{background:none;border:0;padding:0;color:var(--ink);font-size:.855rem;line-height:1.62}

table{border-collapse:collapse;width:100%;margin:1.5em 0;font-size:.945rem}
th,td{text-align:left;padding:.62rem .8rem;border-bottom:1px solid var(--ridge);vertical-align:top}
th{font-weight:600;color:var(--ink-2);font-size:.82rem;letter-spacing:.06em;text-transform:uppercase}
tbody tr{transition:background .16s var(--ease)}
tbody tr:hover{background:color-mix(in srgb,var(--gold) 4%,transparent)}

/* --------------------------------------------------------------- footer */
footer{border-top:1px solid var(--ridge);background:var(--dusk);
  padding:2.6rem clamp(1.1rem,4vw,2.5rem);color:var(--ink-3);font-size:.9rem}
footer .inner{max-width:min(76rem,92vw);margin:0 auto;display:flex;gap:1.6rem;
  flex-wrap:wrap;align-items:center;justify-content:space-between}
footer p{margin:0}

/* ---------------------------------------------------------------- motion */
@media (prefers-reduced-motion:reduce){
  *,*::before,*::after{animation-duration:.001ms!important;animation-iteration-count:1!important;
    transition-duration:.001ms!important}
}
/* The consent banner. Fixed to the bottom corner and never modal: the page
   stays readable, and a reader who ignores it is a reader who declined. */
.consent{position:fixed;top:auto;left:auto;right:1rem;bottom:1rem;margin:0;z-index:50;
  width:min(26rem,calc(100vw - 2rem));box-sizing:border-box;
  background:var(--dusk);color:var(--ink);border:1px solid var(--ridge);border-radius:12px;
  padding:1rem 1.1rem;font-size:.92rem;line-height:1.5;
  box-shadow:0 12px 40px rgba(4,7,11,.65)}
.consent:not([open]){display:none}
.consent p{margin:0 0 .85rem}
.consent a{color:var(--gold)}
.consent-actions{display:flex;gap:.6rem;justify-content:flex-end}
.consent .btn{padding:.5rem .95rem;font-size:.9rem;cursor:pointer;
  font-family:inherit;border:1px solid transparent}
.consent .btn-ghost{border-color:var(--slate)}

/* The copy button on a code block. Hidden without JS, because a control that
   cannot work is worse than no control, and most of this site's readers are
   agents that run none of it. */
.code{position:relative}
/* Room for the button, so it never sits on the first line. The commands here
   are long enough to scroll, and the one most worth copying is the one that
   was underneath it. */
.code pre{padding-right:4.6rem}
.code .copy{
  position:absolute;top:.55rem;right:.55rem;
  display:inline-flex;align-items:center;gap:.35rem;
  padding:.3rem .6rem;border-radius:6px;cursor:pointer;
  background:var(--slate);color:var(--ink-2);border:1px solid var(--ridge);
  font:500 .74rem/1 'Archivo',ui-sans-serif,system-ui,sans-serif;letter-spacing:.02em;
  opacity:0;transition:opacity .18s var(--ease),color .18s var(--ease),border-color .18s var(--ease)}
.code:hover .copy,.code .copy:focus-visible{opacity:1}
.code .copy:hover{color:var(--flash);border-color:var(--brass)}
.code .copy[data-done]{opacity:1;color:var(--gold);border-color:var(--brass)}
.no-js .copy,.no-js .page-actions{display:none}
/* Touch has no hover, so the button would never appear. */
@media (hover:none){.code .copy{opacity:1}}

/* The breadcrumb and the two markdown controls, above the title. The JSON-LD
   has claimed a breadcrumb since the SEO work; this is the reader's copy. */
.crumbs{font-size:.82rem;color:var(--ink-3);margin:0 0 .45rem;
  display:flex;align-items:center;gap:.4rem;flex-wrap:wrap}
.crumbs a{color:var(--ink-3);text-decoration:none}
.crumbs a:hover{color:var(--gold)}
.crumbs span[aria-hidden]{opacity:.55}
.crumbs .here{color:var(--ink-2)}
.page-actions{display:flex;gap:.5rem;flex-wrap:wrap;margin:0 0 1.6rem}
.page-actions a,.page-actions button{
  display:inline-flex;align-items:center;gap:.4rem;
  padding:.36rem .7rem;border-radius:6px;cursor:pointer;text-decoration:none;
  background:transparent;color:var(--ink-2);border:1px solid var(--ridge);
  font:500 .8rem/1.1 'Archivo',ui-sans-serif,system-ui,sans-serif;
  transition:color .18s var(--ease),border-color .18s var(--ease)}
.page-actions a:hover,.page-actions button:hover{color:var(--flash);border-color:var(--brass)}
.page-actions button[data-done]{color:var(--gold);border-color:var(--brass)}
.page-actions svg{width:14px;height:14px;flex:none}

/* The GitHub mark, wherever a link points at the repository. */
.gh{width:1em;height:1em;vertical-align:-.14em;flex:none}
.site-header nav a .gh,footer .gh{margin-right:.32em}

/* The rail marks where you are. Without it the column is a list of links
   that never changes while the page moves under it. */
.rail nav a.here{color:var(--flash)}
.rail nav a.here::before{background:var(--gold)}

/* The DBHQ menu in the home header. A <details>, so it opens with no
   JavaScript; the panel is absolutely positioned so opening it does not
   push the header's own links sideways. */
.org-menu{position:relative;display:inline-block}
.org-menu>summary{
  display:inline-flex;align-items:center;gap:.28rem;cursor:pointer;list-style:none;
  color:var(--ink-2);transition:color .2s var(--ease)}
.org-menu>summary::-webkit-details-marker{display:none}
.org-menu>summary:hover,.org-menu[open]>summary{color:var(--flash)}
.org-menu>summary svg{width:13px;height:13px;transition:transform .2s var(--ease)}
.org-menu[open]>summary svg{transform:rotate(180deg)}
.org-panel{
  position:absolute;right:0;top:calc(100% + .7rem);z-index:40;
  width:min(20rem,calc(100vw - 2rem));
  display:flex;flex-direction:column;
  background:var(--dusk);border:1px solid var(--ridge);border-radius:10px;
  padding:.4rem;box-shadow:0 16px 44px rgba(4,7,11,.7)}
.org-panel a{
  display:block;padding:.55rem .65rem;border-radius:7px;text-decoration:none;
  color:var(--ink);line-height:1.35}
.org-panel a:hover{background:var(--slate)}
.org-panel a b{display:block;font-weight:600;font-size:.94rem}
.org-panel a span{display:block;color:var(--ink-3);font-size:.82rem;margin-top:.1rem}
.org-panel .org-all{
  margin-top:.25rem;border-top:1px solid var(--ridge);border-radius:0 0 7px 7px;
  color:var(--gold);font-size:.88rem;font-weight:500}
/* The panel stays anchored to the menu's RIGHT edge at every width. An
   earlier left:0 override for narrow screens ran it off the side of a phone:
   the menu sits near the right edge, so a panel growing rightwards from there
   has nowhere to go. */

`

// HeroJS draws the signal.
//
// A heliograph does not emit one smooth beam. It flashes: a shutter opens and
// closes, and the message is in the rhythm. So the signal is a TRAIN of pulses
// crossing the valley, not a line being drawn, and the far station answers with
// a shorter one. That is the difference between an animation about light and an
// animation about signalling.
//
// Canvas rather than SVG: dozens of glowing pulses with additive blending are
// what canvas is for, and the terrain is generated rather than drawn.
//
// prefers-reduced-motion paints one frame mid-exchange. It is still legible and
// still says what the product does; it simply does not move.
const HeroJS = `
(function(){
  var c=document.getElementById('signal'); if(!c) return;
  var ctx=c.getContext('2d'), dpr=Math.min(window.devicePixelRatio||1,2);
  var W=0,H=0,ridges=[],stars=[],haze=[],t0=performance.now();
  var reduced=window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  // The message, as a heliograph would send it. Dots and dashes, because the
  // rhythm is the whole point: "HG" in Morse.
  var CODE=[1,1,1,1, 0, 2,2,1];   // 1 = dot, 2 = dash, 0 = word gap
  var REPLY=[2,1,2];

  function resize(){
    var r=c.getBoundingClientRect();
    if(!r.width) return;
    W=r.width; H=r.height;
    c.width=W*dpr; c.height=H*dpr;
    ctx.setTransform(dpr,0,0,dpr,0,0);
    build();
    if(reduced) requestAnimationFrame(frame);
  }

  function build(){
    ridges=[];
    var cfg=[[0.62,0.085,'#0A1216','rgba(78,127,179,.24)'],
             [0.75,0.105,'#080F13','rgba(78,127,179,.34)'],
             [0.90,0.075,'#060B0E','rgba(78,127,179,.50)']];
    for(var k=0;k<cfg.length;k++){
      var pts=[], n=64, base=H*cfg[k][0], amp=H*cfg[k][1], seed=k*29.3+3;
      for(var i=0;i<=n;i++){
        var x=i/n;
        var y=base-amp*(0.5*Math.sin(x*2.7+seed)+0.3*Math.sin(x*6.1+seed*1.9)
                       +0.2*Math.sin(x*11.7+seed*2.7));
        pts.push([x*W,y]);
      }
      ridges.push({pts:pts,fill:cfg[k][2],line:cfg[k][3]});
    }
    stars=[]; for(var j=0;j<64;j++) stars.push([Math.random()*W,Math.random()*H*0.6,Math.random()*0.9+0.25]);
    // Valley mist, which is what gives the distance a middle.
    haze=[]; for(var m=0;m<7;m++) haze.push({x:Math.random()*W,y:H*(0.60+Math.random()*0.22),
      w:W*(0.22+Math.random()*0.30),h:H*(0.030+Math.random()*0.045),v:0.004+Math.random()*0.010});
  }

  // Where a pulse train sits at time p (0..1 across the whole message).
  // Returns the list of pulses currently in flight, each with its own position
  // and brightness, so the beam reads as light travelling rather than a bar.
  function pulses(code,p,speed){
    var out=[], unit=1/(code.length*3+3), t=0;
    for(var i=0;i<code.length;i++){
      var len=code[i]===2?unit*2.2:(code[i]===1?unit*0.9:0);
      if(code[i]!==0){
        var head=(p-t)*speed, tail=(p-t-len)*speed;
        if(head>0&&tail<1) out.push([Math.min(head,1),Math.max(tail,0)]);
      }
      t+=len+unit*0.7;
    }
    return out;
  }

  function frame(now){
    if(!W||!H||ridges.length<3){ requestAnimationFrame(frame); return; }
    var el=(now-t0)/1000;
    ctx.clearRect(0,0,W,H);

    var sky=ctx.createLinearGradient(0,0,0,H);
    sky.addColorStop(0,'#070A10'); sky.addColorStop(0.5,'#08111C');
    sky.addColorStop(0.8,'#0D1B2C'); sky.addColorStop(1,'#070C14');
    ctx.fillStyle=sky; ctx.fillRect(0,0,W,H);

    var sx=W*0.80, sy=H*0.70, sr=Math.max(W,H)*0.50;
    var glow=ctx.createRadialGradient(sx,sy,0,sx,sy,sr);
    glow.addColorStop(0,'rgba(123,167,212,.17)');
    glow.addColorStop(0.35,'rgba(123,167,212,.06)');
    glow.addColorStop(1,'rgba(123,167,212,0)');
    ctx.fillStyle=glow; ctx.fillRect(0,0,W,H);

    for(var s2=0;s2<stars.length;s2++){
      var st=stars[s2];
      ctx.globalAlpha=st[2]*0.42*(0.7+0.3*Math.sin(el*0.7+st[0]));
      ctx.fillStyle='#E6F1FB'; ctx.fillRect(st[0],st[1],1.2,1.2);
    }
    ctx.globalAlpha=1;

    for(var k=0;k<ridges.length;k++){
      var R=ridges[k], pp=R.pts;
      ctx.beginPath(); ctx.moveTo(0,H+2); ctx.lineTo(pp[0][0],pp[0][1]);
      for(var i2=1;i2<pp.length;i2++) ctx.lineTo(pp[i2][0],pp[i2][1]);
      ctx.lineTo(W,H+2); ctx.closePath();
      ctx.fillStyle=R.fill; ctx.fill();
      ctx.beginPath(); ctx.moveTo(pp[0][0],pp[0][1]);
      for(var j2=1;j2<pp.length;j2++) ctx.lineTo(pp[j2][0],pp[j2][1]);
      ctx.strokeStyle=R.line; ctx.lineWidth=1.1; ctx.stroke();

      // Mist, drawn between ridges so it sits IN the valley.
      if(k===1){
        for(var hz=0;hz<haze.length;hz++){
          var Hz=haze[hz];
          if(!reduced){ Hz.x+=Hz.v*W*0.016; if(Hz.x-Hz.w>W) Hz.x=-Hz.w; }
          var hg=ctx.createLinearGradient(Hz.x-Hz.w,0,Hz.x+Hz.w,0);
          hg.addColorStop(0,'rgba(168,204,236,0)');
          hg.addColorStop(0.5,'rgba(168,204,236,.045)');
          hg.addColorStop(1,'rgba(168,204,236,0)');
          ctx.fillStyle=hg; ctx.fillRect(Hz.x-Hz.w,Hz.y,Hz.w*2,Hz.h);
        }
      }
    }

    var near=ridges[2].pts;
    var ax=W*0.09, ay=near[Math.round(near.length*0.09)][1]-9;
    var bx=W*0.91, by=near[Math.round(near.length*0.91)][1]-9;

    var CYCLE=11.0, u=reduced?0.26:((el%CYCLE)/CYCLE);
    var outP=(u-0.04)/0.40, backP=(u-0.58)/0.30;

    // Additive blending: where two pulses overlap the light gets brighter,
    // which is how light actually behaves and what stops this reading as paint.
    ctx.save(); ctx.globalCompositeOperation='lighter';

    function train(x1,y1,x2,y2,code,p,speed,col,warm){
      if(p<=0||p>=1.6) return 0;
      var ps=pulses(code,p,speed), landed=0;
      for(var i=0;i<ps.length;i++){
        var h=ps[i][0], t=ps[i][1];
        if(h>=0.999) landed=1;
        var hx=x1+(x2-x1)*h, hy=y1+(y2-y1)*h;
        var tx=x1+(x2-x1)*t, ty=y1+(y2-y1)*t;
        var g=ctx.createLinearGradient(tx,ty,hx,hy);
        g.addColorStop(0,'rgba('+col+',0)');
        g.addColorStop(0.5,'rgba('+col+',.55)');
        g.addColorStop(1,'rgba('+col+',.95)');
        ctx.strokeStyle=g; ctx.lineWidth=warm?2.6:2.1; ctx.lineCap='round';
        ctx.beginPath(); ctx.moveTo(tx,ty); ctx.lineTo(hx,hy); ctx.stroke();
        var r=warm?15:12, rg=ctx.createRadialGradient(hx,hy,0,hx,hy,r);
        rg.addColorStop(0,'rgba('+col+',.85)');
        rg.addColorStop(0.35,'rgba('+col+',.25)');
        rg.addColorStop(1,'rgba('+col+',0)');
        ctx.fillStyle=rg; ctx.beginPath(); ctx.arc(hx,hy,r,0,7); ctx.fill();
      }
      return landed;
    }

    var arrived=train(ax,ay,bx,by,CODE,outP,1.0,'230,241,251',true);
    var returned=train(bx,by,ax,ay,REPLY,backP,1.0,'123,167,212',false);
    ctx.restore();

    // A landing flash: the far side has received something. This is the moment
    // the whole scene exists to show, so it gets its own ring rather than just
    // a brighter dot.
    function land(x,y,p){
      if(p<=0||p>=1) return;
      var e=1-Math.pow(1-p,3), r=6+e*34;
      ctx.beginPath(); ctx.arc(x,y,r,0,7);
      ctx.strokeStyle='rgba(168,204,236,'+(0.5*(1-p))+')';
      ctx.lineWidth=1.6; ctx.stroke();
    }
    land(bx,by,(u-0.42)/0.14);
    land(ax,ay,(u-0.92)/0.12);

    function station(x,y,lit,label){
      var r=lit?30:18;
      var g=ctx.createRadialGradient(x,y,0,x,y,r);
      g.addColorStop(0,lit?'rgba(223,251,246,.5)':'rgba(123,167,212,.26)');
      g.addColorStop(1,'rgba(123,167,212,0)');
      ctx.fillStyle=g; ctx.beginPath(); ctx.arc(x,y,r,0,7); ctx.fill();
      ctx.beginPath(); ctx.arc(x,y,lit?5:3.8,0,7);
      ctx.fillStyle=lit?'#E6F1FB':'#7BA7D4'; ctx.fill();
      ctx.beginPath(); ctx.moveTo(x,y+4); ctx.lineTo(x,y+14);
      ctx.strokeStyle='rgba(78,127,179,.7)'; ctx.lineWidth=1.4; ctx.stroke();
      if(label){
        ctx.font='500 10px "JetBrains Mono",monospace';
        ctx.fillStyle='rgba(163,180,198,.62)'; ctx.textAlign='center';
        ctx.fillText(label,x,y+29);
      }
    }
    station(ax,ay, u<0.05||returned===1, 'control');
    station(bx,by, arrived===1, 'station');

    if(!reduced) requestAnimationFrame(frame);
  }

  var ro=new ResizeObserver(resize); ro.observe(c);
  resize();
  requestAnimationFrame(frame);
})();
`

// NavJS drives the mobile drawer, and nothing else on the page depends on it.
//
// A native modal dialog rather than a checkbox, details/summary or :has().
// Those three can all be made to LOOK like a drawer and none of them supplies
// what a drawer actually needs: an inert background, contained focus, and
// Escape. showModal gives all three from the platform, and the alternatives end
// up hand-building the same behaviour worse.
//
// It is progressive enhancement in both directions. Without JS, and on a
// browser with no dialog support, the class is put back to no-js and the
// sidebar stays visible above the article as an ordinary grouped list. That is
// less polished and entirely usable, which is the right way round.

// GitHubMark is the Octocat, inlined and inheriting currentColor.
//
// On a link to the repository the mark is recognised before the word next to
// it is read, and it costs no request. Used under GitHub's logo guidance: to
// point at GitHub, unmodified except in size and colour.
const GitHubMark = `<svg class="gh" viewBox="0 0 16 16" aria-hidden="true" focusable="false">` +
	`<path fill="currentColor" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 ` +
	`0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 ` +
	`1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 ` +
	`0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 ` +
	`0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>`

// CopyJS backs both copy controls: the button on every code block, and
// "Copy as markdown" at the top of a docs page.
//
// The markdown one fetches the page's own .md mirror rather than scraping the
// DOM back into markdown. The mirror is byte for byte the source, and the DOM
// is not.
const CopyJS = `
(function(){
  if(!navigator.clipboard)return;
  function flash(btn,word){
    var was=btn.querySelector('span');
    if(!was)return;
    var old=was.textContent;
    was.textContent=word;
    btn.setAttribute('data-done','');
    setTimeout(function(){was.textContent=old;btn.removeAttribute('data-done');},1600);
  }
  document.addEventListener('click',function(e){
    var btn=e.target.closest('[data-copy],[data-copy-markdown]');
    if(!btn)return;
    var md=btn.getAttribute('data-copy-markdown');
    if(md){
      fetch(md).then(function(r){return r.ok?r.text():Promise.reject();})
        .then(function(text){return navigator.clipboard.writeText(text);})
        .then(function(){flash(btn,'Copied');})
        .catch(function(){flash(btn,'Failed');});
      return;
    }
    var code=btn.parentNode.querySelector('code');
    if(!code)return;
    navigator.clipboard.writeText(code.innerText)
      .then(function(){flash(btn,'Copied');})
      .catch(function(){flash(btn,'Failed');});
  });
})();
`

// RailJS marks the section you are reading in the "on this page" column.
//
// An IntersectionObserver rather than a scroll handler: the browser does the
// work off the main thread, and a scroll listener on a long docs page is the
// classic way to make a page feel heavy while doing nothing visible.
const RailJS = `
(function(){
  if(!('IntersectionObserver' in window))return;
  var io=null;
  function bind(){
  if(io){io.disconnect();io=null;}
  var rail=document.querySelector('.rail nav');
  if(!rail)return;
  var links={},ids=[];
  Array.prototype.forEach.call(rail.querySelectorAll('a'),function(a){
    var id=decodeURIComponent(a.getAttribute('href').slice(1));
    if(document.getElementById(id)){links[id]=a;ids.push(id);}
  });
  if(!ids.length)return;
  var seen={};
  function paint(){
    var current=null;
    for(var i=0;i<ids.length;i++){if(seen[ids[i]]){current=ids[i];break;}}
    // Nothing on screen means every heading is above the viewport: keep the
    // last one passed, rather than clearing the mark on a long section.
    if(!current)return;
    for(var j=0;j<ids.length;j++)links[ids[j]].classList.toggle('here',ids[j]===current);
  }
  io=new IntersectionObserver(function(entries){
    entries.forEach(function(en){seen[en.target.id]=en.isIntersecting;});
    paint();
  },{rootMargin:'0px 0px -70% 0px'});
  ids.forEach(function(id){io.observe(document.getElementById(id));});
  }
  bind();
  // The client-side navigation replaces main and the rail wholesale.
  document.addEventListener('hg:swap',bind);
})();
`

// OrgJS closes the DBHQ menu on an outside click or Escape. The one thing
// <details> does not do on its own, and whose absence reads as broken.
const OrgJS = `
(function(){
  var menu=document.querySelector('.org-menu');
  if(!menu)return;
  document.addEventListener('click',function(e){
    if(menu.open&&!menu.contains(e.target))menu.open=false;
  });
  document.addEventListener('keydown',function(e){
    if(e.key==='Escape'&&menu.open){menu.open=false;menu.querySelector('summary').focus();}
  });
})();
`

const NavJS = `<script>
(function(){
  // Any bail-out puts the page back into no-JS mode, because the js class is
  // what HIDES the ordinary sidebar. Returning while still in js leaves a
  // mobile reader with a Menu button that does nothing and no navigation.
  function giveUp(){
    document.documentElement.classList.remove('js');
    document.documentElement.classList.add('no-js');
  }
  var dialog=document.getElementById('docs-menu');
  var opener=document.getElementById('docs-menu-open');
  if(!dialog||!opener){giveUp();return;}
  if(typeof dialog.showModal!=='function'){giveUp();return;}

  var closer=dialog.querySelector('[data-close-menu]');
  var panel=dialog.querySelector('.nav-dialog-panel');
  var side=document.querySelector('.side');
  var desktop=window.matchMedia('(min-width: 64rem)');

  function current(root){return root?root.querySelector('[aria-current="page"]'):null;}

  // ONLY WHEN IT IS ACTUALLY OUT OF VIEW. Centring unconditionally scrolled
  // the brand off the top of the sidebar on every page whose entry was near
  // the start, which is a worse first impression than the problem it fixes.
  function reveal(box,item){
    if(!box||!item||!box.clientHeight)return;
    var b=box.getBoundingClientRect(),i=item.getBoundingClientRect();
    if(i.top>=b.top&&i.bottom<=b.bottom)return;
    box.scrollTop=Math.max(0,i.top-b.top+box.scrollTop-box.clientHeight/2+i.height/2);
  }

  opener.addEventListener('click',function(){
    dialog.showModal();
    opener.setAttribute('aria-expanded','true');
    if(closer)closer.focus({preventScroll:true});
    window.requestAnimationFrame(function(){reveal(panel,current(dialog));});
  });
  if(closer)closer.addEventListener('click',function(){dialog.close();});
  dialog.addEventListener('click',function(e){
    // The dialog fills the viewport and the panel is inside it, so a click
    // landing on the dialog itself is a click on the backdrop.
    if(e.target===dialog){dialog.close();return;}
    if(e.target.closest&&e.target.closest('a'))dialog.close();
  });
  dialog.addEventListener('close',function(){
    opener.setAttribute('aria-expanded','false');
    // Focus has to land somewhere a keyboard can see. Below the breakpoint
    // that is the hamburger it came from; above it the hamburger is
    // display:none and cannot take focus, so the native restoration drops
    // focus to the body and the reader is back at the top of the document
    // with no idea why.
    if(!desktop.matches){opener.focus();return;}
    var c=current(side);
    if(c)c.focus({preventScroll:true});
  });
  function atDesktop(e){
    // An open drawer that survives a resize is an invisible modal holding
    // focus over a layout that no longer has a hamburger to close it with.
    if(e.matches&&dialog.open)dialog.close();
    if(e.matches)reveal(side,current(side));
  }
  atDesktop(desktop);
  desktop.addEventListener('change',atDesktop);

  // ---------------------------------------------------------------- routing
  // THE SIDEBAR STAYS PUT. Every documentation link was an ordinary full page
  // load, so the sidebar was destroyed and rebuilt on every click: it flashed,
  // and worse, its own scroll position went back to the top. On a
  // twenty-three item list that means the reader loses their place in the
  // navigation every single time they use the navigation.
  //
  // So a same-origin docs link swaps only the article and the page rail. The
  // sidebar element is never touched, which is what makes it stay still -
  // nothing here has to remember or restore a scroll offset, because nothing
  // scrolls it.
  //
  // Progressive enhancement throughout. No fetch, no DOMParser, no History
  // API, a cross-origin link, a modified click or a download: the browser does
  // what it always did.
  var main=document.getElementById('main-content');
  var railBox=document.querySelector('.rail');
  if(!main||!window.fetch||!window.DOMParser||!window.history.pushState)return;

  var loading=false;

  function samePage(url){
    return url.pathname===window.location.pathname;
  }

  function swap(html,url,push){
    var doc=new DOMParser().parseFromString(html,'text/html');
    var nextMain=doc.getElementById('main-content');
    if(!nextMain)return false;                     // not a docs page: let the browser have it

    main.innerHTML=nextMain.innerHTML;
    var nextRail=doc.querySelector('.rail');
    if(railBox)railBox.innerHTML=nextRail?nextRail.innerHTML:'';
    document.title=doc.title;

    // The sidebar is NOT replaced. Only the one attribute that says where we
    // are moves, in both copies, so the drawer agrees with the sidebar.
    // Both forms are normalised. Pages serves /azure and /azure/index.html for
    // the same page and the sidebar links the extensionless form, so a reader
    // who arrived on /azure.html would otherwise have no entry marked at all.
    function norm(path){return path.replace(/\/index\.html$/,'').replace(/\.html$/,'').replace(/\/$/,'')||'/';}
    var wanted=norm(url.pathname);
    var links=document.querySelectorAll('.side-nav a');
    for(var i=0;i<links.length;i++){
      var href=norm(new URL(links[i].getAttribute('href'),window.location.origin).pathname);
      if(href===wanted){links[i].setAttribute('aria-current','page');links[i].classList.add('here');}
      else{links[i].removeAttribute('aria-current');links[i].classList.remove('here');}
    }
    if(push)window.history.pushState({hg:1},'',url.href);
    window.scrollTo(0,0);
    // Focus the article, or a keyboard user is left where the link was, in a
    // sidebar whose content no longer relates to the page on screen.
    main.focus({preventScroll:true});
    reveal(side,current(side));
    // Anything bound to the old main or the old rail is now pointing at nodes
    // that left the document. swap() announces; whatever needs rebinding
    // listens, so the next thing that needs it does not have to edit this.
    document.dispatchEvent(new CustomEvent('hg:swap'));
    return true;
  }

  function go(url,push){
    if(loading)return;
    loading=true;
    fetch(url.href,{credentials:'same-origin'}).then(function(r){
      if(!r.ok)throw new Error('status '+r.status);
      return r.text();
    }).then(function(html){
      var done=function(){ if(!swap(html,url,push))window.location.href=url.href; };
      // A view transition where the browser has one, and a plain swap where it
      // does not. Never required: this is the polish, not the mechanism.
      if(document.startViewTransition)document.startViewTransition(done);
      else done();
      loading=false;
    }).catch(function(){
      loading=false;
      window.location.href=url.href;             // never strand the reader
    });
  }

  document.addEventListener('click',function(e){
    if(e.defaultPrevented||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey)return;
    var a=e.target.closest&&e.target.closest('a');
    if(!a||!a.href||a.target||a.hasAttribute('download'))return;
    var url=new URL(a.href);
    if(url.origin!==window.location.origin)return;
    if(url.pathname.indexOf('.md')>-1)return;    // the markdown mirror is a file, not a page
    if(url.hash&&samePage(url))return;           // an in-page anchor is the browser's job
    if(samePage(url))return;
    e.preventDefault();
    if(dialog.open)dialog.close();
    go(url,true);
  });

  window.addEventListener('popstate',function(){
    go(new URL(window.location.href),false);
  });
})();
</script>`
