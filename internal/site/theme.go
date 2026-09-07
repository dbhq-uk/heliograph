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
@font-face{font-family:'Instrument Serif';src:url('/assets/fonts/InstrumentSerif-400.woff2')format('woff2');
  font-weight:400;font-style:normal;font-display:swap}
@font-face{font-family:'Instrument Sans';src:url('/assets/fonts/InstrumentSans.woff2')format('woff2');
  font-weight:400 700;font-style:normal;font-display:swap}
@font-face{font-family:'IBM Plex Mono';src:url('/assets/fonts/IBMPlexMono-400.woff2')format('woff2');
  font-weight:400;font-style:normal;font-display:swap}
@font-face{font-family:'IBM Plex Mono';src:url('/assets/fonts/IBMPlexMono-500.woff2')format('woff2');
  font-weight:500;font-style:normal;font-display:swap}

/* --------------------------------------------------------------- tokens */
:root{
  --night:#08090B;          /* the valley */
  --dusk:#0E1013;           /* raised surfaces */
  --ridge:#171A1F;          /* borders, edges */
  --slate:#252A31;

  --brass:#A87A2E;          /* the instrument */
  --gold:#E8B04B;           /* the signal */
  --flash:#FFF6E0;          /* the light itself. The only pure bright. */

  --ink:#EDEEF0;
  --ink-2:#A7ADB6;          /* 7.1:1 on --night */
  --ink-3:#767D87;          /* 4.6:1 on --night, for captions only */

  --measure:68ch;
  --ease:cubic-bezier(.16,1,.3,1);
}

/* ----------------------------------------------------------------- base */
*,*::before,*::after{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{
  margin:0;background:var(--night);color:var(--ink);
  font:400 17px/1.65 'Instrument Sans',ui-sans-serif,system-ui,sans-serif;
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
  padding:.85rem clamp(1.1rem,4vw,2.5rem);
  background:color-mix(in srgb,var(--night) 82%,transparent);
  backdrop-filter:blur(14px) saturate(1.3);
  border-bottom:1px solid var(--ridge);
}
.brand{display:flex;align-items:center;gap:.6rem;text-decoration:none;color:var(--ink);
  font-family:'Instrument Serif',serif;font-size:1.32rem;letter-spacing:-.01em}
.brand:hover{color:var(--ink)}
.brand svg{width:26px;height:26px;color:var(--gold);flex:none}
nav{display:flex;gap:1.35rem;flex-wrap:wrap;margin-left:auto}
nav a{color:var(--ink-2);text-decoration:none;font-size:.93rem;position:relative;padding:.15rem 0}
nav a::after{content:'';position:absolute;left:0;right:100%;bottom:-2px;height:1px;
  background:var(--gold);transition:right .3s var(--ease)}
nav a:hover{color:var(--ink)}
nav a:hover::after,nav a.here::after{right:0}
nav a.here{color:var(--flash)}

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
  font-family:'Instrument Serif',serif;font-weight:400;
  font-size:clamp(2.9rem,7.4vw,5.6rem);line-height:.98;letter-spacing:-.022em;
  margin:0 0 1.15rem;max-width:16ch;text-wrap:balance;
}
.hero h1 em{font-style:italic;color:var(--gold)}
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
.btn-primary{background:var(--gold);color:#1A1206;
  box-shadow:0 1px 2px rgba(0,0,0,.5),0 10px 26px -12px rgba(232,176,75,.7)}
.btn-primary:hover{background:var(--flash);color:#1A1206;transform:translateY(-1px);
  box-shadow:0 2px 4px rgba(0,0,0,.5),0 16px 34px -12px rgba(232,176,75,.85)}
.btn-ghost{color:var(--ink);border:1px solid var(--slate);background:color-mix(in srgb,var(--dusk) 70%,transparent)}
.btn-ghost:hover{color:var(--flash);border-color:var(--brass);transform:translateY(-1px)}

/* ------------------------------------------------------------ the strip */
/* A real captured log, with a real gap in the timestamp column. The single
   most distinctive fact about this product, shown rather than described. */
.strip{border-bottom:1px solid var(--ridge);background:var(--dusk)}
.strip-inner{max-width:min(76rem,92vw);margin:0 auto;padding:2.6rem clamp(1.1rem,4vw,2.5rem)}
.strip h2{font-size:.78rem;letter-spacing:.14em;text-transform:uppercase;color:var(--ink-3);
  font-weight:500;margin:0 0 1.1rem}
.log{font-family:'IBM Plex Mono',ui-monospace,monospace;font-size:.845rem;line-height:1.95;
  font-variant-numeric:tabular-nums;overflow-x:auto;margin:0}
.log .t{color:var(--ink-3)}
.log .gap{color:var(--gold);background:linear-gradient(90deg,
  color-mix(in srgb,var(--gold) 13%,transparent),transparent 62%);
  display:block;padding-left:.4rem;margin-left:-.4rem;border-radius:3px}

/* ----------------------------------------------------------------- main */
main{max-width:var(--measure);margin:0 auto;padding:3.4rem clamp(1.1rem,4vw,2.5rem) 6rem}
main.wide{max-width:min(76rem,92vw)}
h1,h2,h3,h4{text-wrap:balance}
main h1{font-family:'Instrument Serif',serif;font-weight:400;
  font-size:clamp(2.3rem,5vw,3.4rem);line-height:1.06;letter-spacing:-.02em;margin:.2em 0 .5em}
main h2{font-family:'Instrument Serif',serif;font-weight:400;
  font-size:clamp(1.55rem,3vw,2.05rem);line-height:1.2;letter-spacing:-.012em;
  margin:2.9em 0 .6em;padding-top:1.5rem;border-top:1px solid var(--ridge)}
main h3{font-size:1.1rem;font-weight:600;margin:2.1em 0 .45em;letter-spacing:-.005em}
main h4{font-size:.98rem;font-weight:600;color:var(--ink-2);margin:1.7em 0 .35em}
p,li{margin:.85em 0;text-wrap:pretty}
strong{font-weight:600;color:var(--flash)}
em{color:var(--ink)}
ul{padding-left:1.15rem}
li::marker{color:var(--brass)}

code{font-family:'IBM Plex Mono',ui-monospace,monospace;font-size:.88em;
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
// Canvas rather than SVG: the beam is a gradient sweeping a path with a glow
// that has to stay smooth at 60fps on a laptop, and the terrain is generated
// rather than drawn.
//
// prefers-reduced-motion paints one frame of the scene mid-signal. It is still
// legible and still says what the product does; it simply does not move.
const HeroJS = `
(function(){
  var c=document.getElementById('signal'); if(!c) return;
  var ctx=c.getContext('2d'), dpr=Math.min(window.devicePixelRatio||1,2);
  var W=0,H=0,ridges=[],stars=[],t0=performance.now();
  var reduced=window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function resize(){
    var r=c.getBoundingClientRect();
    if(!r.width) return;
    W=r.width; H=r.height;
    c.width=W*dpr; c.height=H*dpr;
    ctx.setTransform(dpr,0,0,dpr,0,0);
    build();
    // Repaint after a resize. Under reduced motion nothing else will: the one
    // frame has already run, so a later ResizeObserver callback would rebuild
    // the terrain and leave the canvas blank. Which is exactly what it did -
    // the animated path looked fine and the accessible path showed nothing.
    if(reduced) requestAnimationFrame(frame);
  }

  // Three ridges. Two read as a backdrop; three read as distance, which is the
  // whole subject.
  function build(){
    ridges=[];
    var cfg=[[0.62,0.085,'#0B0D11','rgba(168,122,46,.16)'],
             [0.75,0.105,'#090A0D','rgba(168,122,46,.26)'],
             [0.90,0.075,'#060709','rgba(168,122,46,.40)']];
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
    stars=[];
    for(var j=0;j<58;j++) stars.push([Math.random()*W,Math.random()*H*0.62,Math.random()*0.9+0.25]);
  }

  function frame(now){
    // The first frame can beat the first layout: resize() bails when the
    // element still has no width, which leaves ridges empty and made the whole
    // scene throw on ridges[2]. A blank hero and a working one look identical
    // in a screenshot, so this is guarded rather than assumed.
    if(!W||!H||ridges.length<3){ requestAnimationFrame(frame); return; }
    var el=(now-t0)/1000;
    ctx.clearRect(0,0,W,H);

    // A valley at dusk, warmer toward the horizon where the sun has gone.
    var sky=ctx.createLinearGradient(0,0,0,H);
    sky.addColorStop(0,'#08090B'); sky.addColorStop(0.52,'#0A0C10');
    sky.addColorStop(0.78,'#12100E'); sky.addColorStop(1,'#0A0B0D');
    ctx.fillStyle=sky; ctx.fillRect(0,0,W,H);

    // The sun, low and to the right: the source the mirror is catching.
    var sx=W*0.80, sy=H*0.66, sr=Math.max(W,H)*0.45;
    var glow=ctx.createRadialGradient(sx,sy,0,sx,sy,sr);
    glow.addColorStop(0,'rgba(232,176,75,.17)');
    glow.addColorStop(0.35,'rgba(232,176,75,.06)');
    glow.addColorStop(1,'rgba(232,176,75,0)');
    ctx.fillStyle=glow; ctx.fillRect(0,0,W,H);

    for(var s2=0;s2<stars.length;s2++){
      var st=stars[s2];
      ctx.globalAlpha=st[2]*0.4*(0.7+0.3*Math.sin(el*0.7+st[0]));
      ctx.fillStyle='#FFF6E0';
      ctx.fillRect(st[0],st[1],1.2,1.2);
    }
    ctx.globalAlpha=1;

    for(var k=0;k<ridges.length;k++){
      var R=ridges[k], p=R.pts;
      ctx.beginPath(); ctx.moveTo(0,H+2); ctx.lineTo(p[0][0],p[0][1]);
      for(var i=1;i<p.length;i++) ctx.lineTo(p[i][0],p[i][1]);
      ctx.lineTo(W,H+2); ctx.closePath();
      ctx.fillStyle=R.fill; ctx.fill();
      ctx.beginPath(); ctx.moveTo(p[0][0],p[0][1]);
      for(var j2=1;j2<p.length;j2++) ctx.lineTo(p[j2][0],p[j2][1]);
      ctx.strokeStyle=R.line; ctx.lineWidth=1.1; ctx.stroke();
    }

    var near=ridges[2].pts;
    var ax=W*0.09, ay=near[Math.round(near.length*0.09)][1]-9;
    var bx=W*0.91, by=near[Math.round(near.length*0.91)][1]-9;

    var CYCLE=9.0, u=reduced?0.30:((el%CYCLE)/CYCLE);
    var out=Math.max(0,Math.min(1,(u-0.03)/0.38));
    var back=Math.max(0,Math.min(1,(u-0.55)/0.38));

    function beam(x1,y1,x2,y2,pr,warm){
      if(pr<=0||pr>=1) return;
      var hx=x1+(x2-x1)*pr, hy=y1+(y2-y1)*pr;
      var tail=Math.max(0,pr-0.42), tx=x1+(x2-x1)*tail, ty=y1+(y2-y1)*tail;
      ctx.save();
      ctx.shadowColor=warm?'rgba(255,246,224,.55)':'rgba(232,176,75,.45)';
      ctx.shadowBlur=16;
      var g=ctx.createLinearGradient(tx,ty,hx,hy);
      g.addColorStop(0,'rgba(232,176,75,0)');
      g.addColorStop(1,warm?'rgba(255,246,224,1)':'rgba(232,176,75,.95)');
      ctx.strokeStyle=g; ctx.lineWidth=2.4; ctx.lineCap='round';
      ctx.beginPath(); ctx.moveTo(tx,ty); ctx.lineTo(hx,hy); ctx.stroke();
      ctx.restore();
      var r=16, rg=ctx.createRadialGradient(hx,hy,0,hx,hy,r);
      rg.addColorStop(0,'rgba(255,246,224,.95)');
      rg.addColorStop(0.3,'rgba(255,246,224,.35)');
      rg.addColorStop(1,'rgba(255,246,224,0)');
      ctx.fillStyle=rg; ctx.beginPath(); ctx.arc(hx,hy,r,0,7); ctx.fill();
    }

    function station(x,y,lit,label){
      var r=lit?28:17;
      var g=ctx.createRadialGradient(x,y,0,x,y,r);
      g.addColorStop(0,lit?'rgba(255,246,224,.55)':'rgba(232,176,75,.24)');
      g.addColorStop(1,'rgba(232,176,75,0)');
      ctx.fillStyle=g; ctx.beginPath(); ctx.arc(x,y,r,0,7); ctx.fill();
      ctx.beginPath(); ctx.arc(x,y,lit?5:3.8,0,7);
      ctx.fillStyle=lit?'#FFF6E0':'#C9922F'; ctx.fill();
      // A short mast, so a station reads as a thing somebody put there.
      ctx.beginPath(); ctx.moveTo(x,y+4); ctx.lineTo(x,y+14);
      ctx.strokeStyle='rgba(168,122,46,.6)'; ctx.lineWidth=1.4; ctx.stroke();
      if(label){
        ctx.font='500 10px "IBM Plex Mono",monospace';
        ctx.fillStyle='rgba(167,173,182,.6)'; ctx.textAlign='center';
        ctx.fillText(label,x,y+29);
      }
    }

    beam(ax,ay,bx,by,out,true);
    beam(bx,by,ax,ay,back,false);
    station(ax,ay, out<0.08||back>0.92, 'control');
    station(bx,by, out>0.9&&back<0.06, 'station');

    if(!reduced) requestAnimationFrame(frame);
  }

  var ro=new ResizeObserver(resize); ro.observe(c);
  resize();
  requestAnimationFrame(frame);
})();
`
