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
header{
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
header>nav{display:flex;gap:1.35rem;flex-wrap:nowrap;margin-left:auto}
header>nav a{color:var(--ink-2);text-decoration:none;font-size:.93rem;
  position:relative;padding:.15rem 0;white-space:nowrap}
header>nav a::after{content:'';position:absolute;left:0;right:100%;bottom:-2px;height:1px;
  background:var(--gold);transition:right .3s var(--ease)}
header>nav a:hover{color:var(--ink)}
header>nav a:hover::after{right:0}
/* Below this the three links become a deliberate second row rather than
   wrapping mid-list. A link never breaks across lines. */
@media(max-width:420px){
  header{flex-wrap:wrap;gap:.55rem}
  header>nav{width:100%;margin-left:0;justify-content:space-between}
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
.shell{
  display:grid;
  grid-template-columns:15.5rem minmax(0,1fr) 13.5rem;
  gap:0 2.4rem;
  max-width:90rem;margin:0 auto;
  padding:0 var(--gutter,clamp(1rem,2.2vw,1.8rem));
  align-items:start;
}
.side{
  position:sticky;top:3.6rem;align-self:start;
  max-height:calc(100vh - 3.6rem);overflow-y:auto;
  padding:2.6rem 0 3rem;
  border-right:1px solid var(--ridge);
}
.side nav{display:flex;flex-direction:column;gap:.08rem;margin:0}
.side .grp{
  font-size:.72rem;letter-spacing:.12em;text-transform:uppercase;
  color:var(--ink-3,var(--ink-2));font-weight:600;
  margin:1.5rem 0 .5rem;padding-right:2rem;
}
.side .grp:first-child{margin-top:0}
.side nav a{
  color:var(--ink-2);text-decoration:none;font-size:.92rem;
  padding:.34rem .7rem;margin-right:1.6rem;border-radius:6px;
  line-height:1.35;transition:color .15s var(--ease),background .15s var(--ease);
}
.side nav a::after{display:none}          /* the top nav's underline, not wanted here */
.side nav a:hover{color:var(--ink);background:var(--dusk)}
.side nav a.here{color:var(--ink);background:var(--dusk);font-weight:500}

.col{min-width:0}                          /* so a wide <pre> scrolls instead of stretching the grid */

/* The rail. Quiet by construction: it is a way back to a heading, not a
   second navigation competing with the first. */
.rail{
  position:sticky;top:3.6rem;align-self:start;
  max-height:calc(100vh - 3.6rem);overflow-y:auto;
  padding:3.9rem 0 3rem;
}
.rail .grp{
  font-size:.72rem;letter-spacing:.12em;text-transform:uppercase;
  color:var(--ink-2);font-weight:600;margin:0 0 .6rem;
}
.rail nav{display:flex;flex-direction:column;gap:.05rem;margin:0}
.rail nav a{
  color:var(--ink-2);text-decoration:none;font-size:.85rem;line-height:1.4;
  padding:.25rem 0 .25rem .7rem;border-left:1px solid var(--ridge);
}
.rail nav a::after{display:none}
.rail nav a:hover{color:var(--ink);border-left-color:var(--gold)}

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
.shell main .dgw{max-width:none;width:100%}
@media(max-width:560px){.dg text{font-size:14px}.dg-sub,.dg-foot{font-size:12px}}

/* ----------------------------------------------------------------- main */
/* Inside the shell the column already sets the width, so main only sets the
   measure it will not exceed. --measure is the line length prose is readable
   at; the column is wider than that so tables and code can use the room. */
main{max-width:var(--measure);margin:0;padding:2.6rem 0 6rem}
.shell main{max-width:min(80ch,100%)}
main.wide{max-width:min(76rem,92vw);margin:0 auto;padding:3.4rem clamp(1.1rem,4vw,2.5rem) 6rem}

/* A table or a code block may use the whole column: they are scanned, not
   read, and a 68ch table wraps cells that were meant to line up. */
.shell main table,.shell main pre{max-width:none;width:100%}

@media(max-width:1180px){
  .shell{grid-template-columns:14rem minmax(0,1fr);gap:0 2rem}
  .rail{display:none}                      /* the first column to go: it is the least load-bearing */
}
@media(max-width:820px){
  .shell{grid-template-columns:1fr;padding:0 1.1rem}
  .side{
    position:static;max-height:none;border-right:0;
    border-bottom:1px solid var(--ridge);padding:1.2rem 0;
  }
  .side nav{flex-direction:row;flex-wrap:wrap;gap:.2rem}
  .side .grp{display:none}                 /* group headings in a wrapped row are noise */
  .side nav a{margin-right:0}
  main{padding:2rem 0 4rem}
}
h1,h2,h3,h4{text-wrap:balance}
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
