package site

// MatrixCSS and MatrixJS live apart from the theme's own so that a page
// without a matrix carries neither.
//
// THE COLOUR IS THE ONE PLACE THIS PAGE LEAVES THE HOUSE PALETTE, and it does
// so deliberately. Everything else on the site is denim on a blue-black
// valley, because an accent works by scarcity. A grid of seventy cells has no
// scarcity to spend: five states rendered in one hue differ only by
// brightness, which is the distinction colour-blind readers lose first and
// everybody loses on a dim screen. So the states get hue AND shape AND a
// label, and the hues are measured against --night the way the rest were:
//
//	proven  #6FD08C  9.9:1     needs     #E8B93F  11.4:1
//	works   #7BA7D4  7.8:1     untested  #93A1B5  6.4:1
//	none    #6A4F5C  -         (a fill, never text)
const MatrixCSS = `
/* ------------------------------------------------------------ the matrix */
:root{
  --mx-proven:#6FD08C;
  --mx-works:#7BA7D4;
  --mx-needs:#E8B93F;
  --mx-untested:#93A1B5;
  --mx-none:#2A1F27;
}
.vh{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;
  clip:rect(0 0 0 0);white-space:nowrap;border:0}

/* THIS PAGE IS AN APPLICATION, not a document with a widget in it. The prose
   above the grid was cut to three sentences and the grid takes the room that
   freed: full width of the shell, and the height of the viewport under it. */
.docs-shell--wide{max-width:none;padding:0 clamp(.6rem,2vw,1.6rem)}
.docs-shell--wide main{max-width:none;padding-top:1.2rem;padding-bottom:1.5rem}
.docs-shell--wide .rail{display:none}
.docs-shell--wide main h1{font-size:clamp(1.5rem,3.2vw,2.1rem);margin-bottom:.3rem}
.docs-shell--wide main h1+p{margin:0 0 .9rem;color:var(--ink-3);font-size:.92rem}
.mxa{margin:0 0 1.4rem;display:flex;flex-direction:column;gap:.8rem}

/* -- the toolbar -- */
.mxa-bar{display:flex;flex-wrap:wrap;gap:.8rem 1.4rem;align-items:flex-end;
  padding:.75rem .9rem;border:1px solid var(--ridge);border-radius:12px;background:var(--dusk)}
.mxa-fg{min-width:0}
.mxa-fl{display:block;font-size:.66rem;letter-spacing:.09em;text-transform:uppercase;
  color:var(--ink-3);margin-bottom:.35rem}
.mxa-chips{display:flex;gap:.3rem;flex-wrap:wrap}
.mxa-chip{border:1px solid var(--ridge);background:none;color:var(--ink-2);font:inherit;
  font-size:.8rem;border-radius:999px;padding:.24rem .7rem;cursor:pointer;white-space:nowrap;
  transition:border-color .15s,color .15s,background .15s}
.mxa-chip:hover{border-color:var(--brass);color:var(--ink)}
.mxa-chip:focus-visible{outline:2px solid var(--gold);outline-offset:2px}
.mxa-chip[aria-checked="true"]{background:var(--gold);border-color:var(--gold);color:#06213A;font-weight:600}
.mxa-count{margin:0 0 .1rem auto;font-size:.8rem;color:var(--ink-3);white-space:nowrap}
.mxa-count b{color:var(--flash);font-size:1.45rem;font-weight:600;line-height:1;
  font-variant-numeric:tabular-nums;margin-right:.15rem}

/* One line, and only while that shape is chosen. */
.mxa-def{display:none;margin:0;padding:.55rem .8rem;border-left:3px solid var(--brass);
  background:var(--dusk);font-size:.85rem;color:var(--ink-2);border-radius:0 8px 8px 0}
.mxa-def b{color:var(--flash)}
[data-kindfilter="pigeonhole"] .mxa-def[data-def="pigeonhole"],
[data-kindfilter="intercom"] .mxa-def[data-def="intercom"]{display:block}

/* -- grid beside panel -- */
.mxa-body{display:grid;grid-template-columns:minmax(0,1fr) 19rem;gap:1rem;align-items:stretch}
.mxa-gridwrap{overflow:auto;border:1px solid var(--ridge);border-radius:12px;background:var(--dusk)}

.mxa-grid{border-collapse:separate;border-spacing:0;width:100%;min-width:38rem;margin:0;
  font-size:.9rem;table-layout:fixed}
.mxa-grid td,.mxa-grid th{border:0;padding:0;vertical-align:middle}
.mxa-corner{width:10.5rem;min-width:10.5rem;padding:.45rem .6rem;line-height:1.15;
  position:sticky;left:0;top:0;z-index:4;background:var(--dusk);overflow:hidden}
.mxa-corner span{display:block;font-size:.6rem;letter-spacing:.08em;text-transform:uppercase;color:var(--ink-3)}
.mxa-corner span:last-child{text-align:right;color:var(--brass)}

/* The head row sticks too: scrolling ten rows should not cost you the column
   names, and this grid is meant to be scrolled. */
.mxa-grid thead th{padding:.45rem .25rem;text-align:center;font-size:.72rem;font-weight:600;
  color:var(--ink-2);border-bottom:1px solid var(--ridge);background:var(--dusk);
  position:sticky;top:0;z-index:3;transition:color .15s,background .15s}
.mxa-grid thead th a{color:inherit;text-decoration:none}
.mxa-grid thead th a:hover{text-decoration:underline}
.mxa-grid thead th em{display:block;font-style:normal;font-size:.56rem;letter-spacing:.08em;
  text-transform:uppercase;color:var(--ink-3);margin-top:.1rem}

.mxa-grid tbody th{text-align:left;padding:.4rem .6rem;font-size:.8rem;font-weight:500;
  color:var(--ink-2);text-transform:none;letter-spacing:0;border-right:1px solid var(--ridge);
  transition:color .15s,background .15s;position:sticky;left:0;z-index:2;background:var(--dusk)}
.mxa-grid tbody th a{color:inherit;text-decoration:none}
.mxa-grid tbody th a:hover{text-decoration:underline}
.mxa-grid tbody th em{display:block;font-style:normal;font-size:.6rem;color:var(--ink-3)}
.mxa-grid tbody tr:nth-child(even) td{background:rgba(255,255,255,.012)}

/* -- the cells -- */
.mxa-grid tbody td{text-align:center}
.mxa-grid tbody td button{display:block;width:100%;border:0;background:none;
  padding:.5rem .2rem;cursor:pointer;color:inherit;min-height:2.6rem}
.mxa-grid tbody td button:focus{outline:none}
.mxa-grid tbody td button:focus-visible{outline:2px solid var(--flash);outline-offset:-2px;border-radius:6px}

/* EVERY STATE HAS ITS OWN SHAPE as well as its own hue, because five states
   in one hue differ only by brightness and that is the distinction a
   colour-blind reader loses first. */
.mxa-dot{display:inline-block;width:1rem;height:1rem;border-radius:4px;
  background:var(--mx-none);transition:transform .18s var(--ease),box-shadow .18s}
[data-state="proven"] .mxa-dot{background:var(--mx-proven);border-radius:4px}
[data-state="works"] .mxa-dot{background:var(--mx-works);border-radius:50%}
[data-state="needs"] .mxa-dot{background:none;border:2px solid var(--mx-needs);border-radius:50%}
[data-state="untested"] .mxa-dot{background:none;border:2px dashed var(--mx-untested);border-radius:4px}
[data-state="none"] .mxa-dot{background:none;position:relative;opacity:.5}
[data-state="none"] .mxa-dot:before,[data-state="none"] .mxa-dot:after{
  content:"";position:absolute;inset:45% 12%;background:var(--slate);border-radius:1px}
[data-state="none"] .mxa-dot:before{transform:rotate(45deg)}
[data-state="none"] .mxa-dot:after{transform:rotate(-45deg)}
@media(hover:hover){.mxa-grid tbody td:hover .mxa-dot{transform:scale(1.22)}}

/* -- crosshair and flash -- */
/* A STICKY CELL MUST STAY OPAQUE. This rule set a translucent tint with the
   background shorthand, which threw away the opaque var(--dusk) underneath -
   so on the lit row the label column became see-through and the cells
   scrolling beneath it showed straight through the label. The tint goes on as
   an image layer over an opaque colour instead, which looks identical and
   hides what is behind it. */
.mxa-grid tbody tr[data-lit] th,.mxa-grid thead th[data-lit]{color:var(--flash);
  background-color:var(--dusk);
  background-image:linear-gradient(rgba(123,167,212,.14),rgba(123,167,212,.14))}
.mxa-grid td[data-lit]{background:rgba(123,167,212,.06)}
.mxa-grid td[data-on] .mxa-dot{box-shadow:0 0 0 3px rgba(230,241,251,.22),0 0 16px 2px currentColor;transform:scale(1.3)}
.mxa-grid td[data-on]{background:rgba(123,167,212,.14);box-shadow:inset 0 0 0 1px var(--gold)}

/* The heliograph is a mirror flashing light across a valley. */
@keyframes mx-beam{from{background-position:-40% 0}to{background-position:140% 0}}
.mxa-grid tbody tr[data-lit]{background-image:linear-gradient(90deg,
  transparent 0%,rgba(230,241,251,.10) 45%,rgba(230,241,251,.16) 50%,rgba(230,241,251,.10) 55%,transparent 100%);
  background-size:60% 100%;background-repeat:no-repeat;animation:mx-beam .55s var(--ease) 1}
@media(prefers-reduced-motion:reduce){
  .mxa-grid tbody tr[data-lit]{animation:none;background-image:none}
  .mxa-dot,.mxa-panel{transition:none}
}

/* -- the panel -- */
.mxa-panel{border:1px solid var(--ridge);border-radius:12px;background:var(--dusk);
  padding:.9rem 1rem 1rem;align-self:start;position:sticky;top:1rem}
.mxa-state{margin:0 0 .4rem;font-size:.66rem;letter-spacing:.1em;text-transform:uppercase;
  font-weight:700;display:inline-block;padding:.16rem .5rem;border-radius:5px;color:#06213A}
.mxa-state[data-state="proven"]{background:var(--mx-proven)}
.mxa-state[data-state="works"]{background:var(--mx-works)}
.mxa-state[data-state="needs"]{background:var(--mx-needs)}
.mxa-state[data-state="untested"]{background:var(--mx-untested)}
.mxa-state[data-state="none"]{background:var(--slate);color:var(--ink-2)}
.mxa-panel h3{margin:0 0 .45rem;font-size:1rem;line-height:1.3;color:var(--flash)}
.mxa-panel h3 span{color:var(--ink-3);font-weight:400}
.mxa-panel p[data-panel-note]{margin:0 0 .7rem;font-size:.86rem;color:var(--ink-2);line-height:1.5}
.mxa-meta{margin:0 0 .7rem;display:grid;gap:.28rem;font-size:.76rem}
.mxa-meta div{display:flex;gap:.5rem;justify-content:space-between;border-bottom:1px solid var(--ridge);padding-bottom:.22rem}
.mxa-meta dt{color:var(--ink-3);margin:0}
.mxa-meta dd{margin:0;color:var(--ink-2);text-align:right}
.mxa-links{margin:0 0 .4rem;display:flex;gap:.8rem;flex-wrap:wrap;font-size:.8rem}
.mxa-hint{margin:0;font-size:.72rem;color:var(--ink-3)}
.mxa-close{display:none}

/* -- legend, folded tables, filter feedback -- */
.mxa-legend{list-style:none;display:flex;flex-wrap:wrap;gap:.3rem 1rem;margin:0;padding:0;
  font-size:.76rem;color:var(--ink-3)}
.mxa-legend li{display:flex;align-items:center;gap:.35rem}
.mxa-legend .mxa-dot{width:.75rem;height:.75rem}
.mxa-tables{margin:.4rem 0 0;border:1px solid var(--ridge);border-radius:10px;background:var(--dusk)}
.mxa-tables summary{cursor:pointer;padding:.55rem .9rem;font-size:.86rem;color:var(--ink-2)}
.mxa-tables summary:hover{color:var(--flash)}
.mxa-tables[open] summary{border-bottom:1px solid var(--ridge)}
.mxa-tables .tw{padding:0 .9rem .4rem}
.mxa-grid [data-dim]{opacity:.22}
.mxa-grid [data-dim] .mxa-dot{filter:grayscale(1)}
.mxa-panel[data-hidden]{border-style:dashed;opacity:.75}
.mxa-panel[data-hidden]:before{content:"Filtered out of the grid";display:block;
  font-size:.64rem;letter-spacing:.08em;text-transform:uppercase;color:var(--mx-needs);margin-bottom:.45rem}
.mxa-empty{margin:0;padding:.65rem .9rem;border:1px dashed var(--mx-needs);border-radius:8px;
  color:var(--ink-2);font-size:.86rem}
.mxa-empty[hidden]{display:none}

/* ---------------------------------------------------------------- mobile */
/* A phone gets a genuinely different layout rather than the desktop one made
   small: the grid keeps the full width and the panel becomes a sheet that
   rises when a cell is tapped, because a 19rem column beside a 7-column grid
   on a 380px screen is two things neither of which is usable. */
@media(max-width:62rem){
  .mxa-body{grid-template-columns:1fr}
  .mxa-bar{gap:.6rem 1rem;padding:.6rem .7rem}
  .mxa-fg{flex:1 1 100%}
  /* One scrolling line per group. Wrapping three groups of chips is a
     toolbar taller than the grid it controls. */
  .mxa-chips{flex-wrap:nowrap;overflow-x:auto;padding-bottom:.15rem;
    scrollbar-width:none;-ms-overflow-style:none}
  .mxa-chips::-webkit-scrollbar{display:none}
  .mxa-count{margin:0;order:-1;flex:1 1 100%}
  .mxa-gridwrap{max-height:none}
  .mxa-grid{min-width:34rem;font-size:.84rem}
  .mxa-corner{width:7.5rem;min-width:7.5rem}
  /* "TRANSPORT" does not fit 7.5rem and was spilling over the first column. */
  .mxa-corner span{font-size:.52rem;letter-spacing:.04em}
  .mxa-grid tbody th{padding:.36rem .45rem;font-size:.74rem}
  .mxa-grid tbody th em{display:none}
  .mxa-grid thead th em{display:none}

  .mxa-panel{position:fixed;left:0;right:0;bottom:0;top:auto;z-index:40;
    border-radius:14px 14px 0 0;border-bottom:0;max-height:62vh;overflow-y:auto;
    box-shadow:0 -18px 40px rgba(0,0,0,.55);
    transform:translateY(105%);transition:transform .22s var(--ease);
    padding-bottom:calc(1rem + env(safe-area-inset-bottom))}
  .mxa-panel[data-open]{transform:translateY(0)}
  .mxa-hint{display:none}
  .mxa-close{display:block;width:100%;margin-top:.6rem;border:1px solid var(--ridge);
    background:none;color:var(--ink-2);font:inherit;font-size:.85rem;border-radius:8px;
    padding:.55rem;cursor:pointer}
  .mxa-close:hover{border-color:var(--gold);color:var(--flash)}
  /* A 2.6rem tap target is below what a thumb can hit reliably. */
  .mxa-grid tbody td button{min-height:2.9rem;padding:.55rem .2rem}
}
@media(max-width:26rem){
  .mxa-grid{min-width:30rem}
  .mxa-corner{width:6.2rem;min-width:6.2rem}
  .mxa-corner span:last-child{display:none}
}
`

// MatrixJS drives the crosshair, the panel, the filters and the deep link.
//
// It adds behaviour to a grid that is already complete in the HTML. Nothing
// here builds a cell, and nothing here decides a state - both were settled in
// Go, where a test can reach them.
const MatrixJS = `
(function(){
  var root=document.querySelector('[data-matrix]');
  if(!root)return;
  var raw=document.getElementById('mx-data');
  if(!raw)return;
  var D;
  try{D=JSON.parse(raw.textContent);}catch(e){return;}

  var T={},S={},C={};
  D.t.forEach(function(x){T[x.ID]=x;});
  D.s.forEach(function(x){S[x.ID]=x;});
  D.c.forEach(function(x){C[x.ID]=x;});

  var grid=root.querySelector('.mxa-grid');
  var rows=[].slice.call(grid.querySelectorAll('tbody tr'));
  var heads=[].slice.call(grid.querySelectorAll('thead th[data-col]'));
  var count=root.querySelector('[data-count]');
  var empty=root.querySelector('[data-empty]');

  // The grid is addressed by row and column, never by a flat index with an
  // assumed width. A flat index sends Right on the last column into the next
  // row, and Down into whatever is seven cells along whether or not that is
  // the cell below.
  var G=rows.map(function(r){return [].slice.call(r.querySelectorAll('td[data-cell]'));});
  var cur={r:0,c:0};

  // Filters are initialised FROM the markup, so the rendered chips and the
  // script cannot start out disagreeing about what is selected.
  var filters={};
  [].slice.call(root.querySelectorAll('.mxa-chip')).forEach(function(ch){
    var g=ch.getAttribute('data-f');
    if(ch.getAttribute('aria-checked')==='true'||!(g in filters))
      if(ch.getAttribute('aria-checked')==='true')filters[g]=ch.getAttribute('data-v');
  });

  function el(sel){return root.querySelector(sel);}
  var P={
    state:el('[data-panel-state]'),title:el('[data-panel-title]'),note:el('[data-panel-note]'),
    kind:el('[data-panel-kind]'),flavour:el('[data-panel-flavour]'),control:el('[data-panel-control]'),
    tlink:el('[data-panel-tlink]'),slink:el('[data-panel-slink]'),
    box:el('[data-panel]'),say:el('[data-say]')
  };
  var LABEL={proven:'Proven',works:'Works',needs:'Needs a step',untested:'Never checked',none:'Cannot'};

  function at(r,c){return G[r]&&G[r][c];}
  function pinnedCell(){return at(cur.r,cur.c);}

  function describe(td){
    if(!td)return;
    var key=td.getAttribute('data-cell'),p=key.split('.'),s=S[p[0]],t=T[p[1]],c=D.cells[key];
    if(!s||!t||!c)return;
    P.state.textContent=LABEL[c.s]||c.s;
    P.state.setAttribute('data-state',c.s);
    P.title.textContent='';
    P.title.appendChild(document.createTextNode(s.Name+' '));
    var sp=document.createElement('span');sp.textContent='over';P.title.appendChild(sp);
    P.title.appendChild(document.createTextNode(' '+t.Name));
    var note=c.n||t.Note;
    var ctl=C[filters.ctl];
    if(ctl){
      note+=ctl.tr[t.ID]?' Driven from '+ctl.Name+': '+ctl.tr[t.ID]+'.'
                        :' '+ctl.Name+' cannot drive '+t.Name+' at all.';
    }
    P.note.textContent=note;
    P.kind.textContent=t.Kind;
    P.flavour.textContent=s.Flavour;
    P.control.textContent=t.Control;
    P.tlink.href=t.Href;P.tlink.textContent='About '+t.Name;
    P.slink.href=s.Href;P.slink.textContent='About '+s.Short;
  }

  function crosshair(td,flash){
    rows.forEach(function(r){r.removeAttribute('data-lit');});
    heads.forEach(function(h){h.removeAttribute('data-lit');});
    G.forEach(function(rw){rw.forEach(function(c){c.removeAttribute('data-lit');});});
    if(!td)return;
    var col=td.getAttribute('data-cell').split('.')[1];
    var row=td.parentNode;
    // Reduced motion gets no restart hack: void offsetWidth forces a
    // synchronous layout, and doing that for an animation that will not run
    // is a reflow bought for nothing.
    var motion=!window.matchMedia||!window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    if(flash&&motion){row.removeAttribute('data-lit');void row.offsetWidth;}
    row.setAttribute('data-lit','');
    heads.forEach(function(h){if(h.getAttribute('data-col')===col)h.setAttribute('data-lit','');});
    G.forEach(function(rw){rw.forEach(function(c){
      if(c.getAttribute('data-cell').split('.')[1]===col)c.setAttribute('data-lit','');});});
  }

  function paintPin(){
    var td=pinnedCell();
    G.forEach(function(rw){rw.forEach(function(c){
      c.removeAttribute('data-on');
      var b=c.querySelector('button');
      b.tabIndex=-1;b.setAttribute('aria-pressed','false');
    });});
    if(!td)return;
    td.setAttribute('data-on','');
    var b=td.querySelector('button');
    b.tabIndex=0;b.setAttribute('aria-pressed','true');
    P.box.toggleAttribute('data-hidden',td.hasAttribute('data-dim'));
  }

  function announce(){
    // Re-selecting the same cell writes identical text, and an unchanged live
    // region is a region a screen reader may say nothing about. Clearing
    // first, on the next frame, makes the second selection a real change.
    P.say.textContent='';
    var msg=P.state.textContent+'. '+P.title.textContent+'. '+P.note.textContent;
    setTimeout(function(){P.say.textContent=msg;},60);
  }

  function pin(r,c,opts){
    var td=at(r,c);
    if(!td)return;
    cur={r:r,c:c};
    describe(td);
    crosshair(td,true);
    paintPin();
    if(history.replaceState)
      history.replaceState(null,'','#'+encodeURIComponent(td.getAttribute('data-cell')));
    if(opts&&opts.focus)td.querySelector('button').focus();
    // The sheet rises only when a person chose a cell. Restoring a deep link
    // on load must not cover the grid before they have seen it.
    if(opts&&opts.open)P.box.setAttribute('data-open','');
    announce();
  }

  function applyFilters(){
    var n=0,anyVisible=false;
    heads.forEach(function(h){
      var id=h.getAttribute('data-col'),t=T[id],off=false;
      if(filters.kind!=='all'&&t.Kind!==filters.kind)off=true;
      if(filters.ctl!=='any'&&C[filters.ctl]&&!C[filters.ctl].tr[id])off=true;
      h.toggleAttribute('data-dim',off);
    });
    G.forEach(function(rw,ri){
      var shown=0,runs=0;
      rw.forEach(function(td){
        var id=td.getAttribute('data-cell').split('.')[1],st=td.getAttribute('data-state'),off=false;
        if(filters.kind!=='all'&&T[id].Kind!==filters.kind)off=true;
        if(filters.ctl!=='any'&&C[filters.ctl]&&!C[filters.ctl].tr[id])off=true;
        if(filters.only==='runs'&&st==='none')off=true;
        if(filters.only==='proven'&&st!=='proven')off=true;
        td.toggleAttribute('data-dim',off);
        // A row is dimmed when the FILTERS leave nothing in it, not when
        // nothing in it runs. A row of honest "cannot" cells still answers
        // the question the reader asked.
        if(!off){shown++;anyVisible=true;if(st!=='none')runs++;}
      });
      n+=runs;
      rows[ri].toggleAttribute('data-dim',shown===0);
    });
    count.querySelector('b').textContent=n;
    if(empty)empty.hidden=anyVisible;
    var td=pinnedCell();
    if(td)P.box.toggleAttribute('data-hidden',td.hasAttribute('data-dim'));
  }

  root.addEventListener('click',function(e){
    var chip=e.target.closest('.mxa-chip');
    if(chip){
      var g=chip.getAttribute('data-f');
      if(!(g in filters))return;
      filters[g]=chip.getAttribute('data-v');
      [].slice.call(root.querySelectorAll('.mxa-chip[data-f="'+g+'"]')).forEach(function(c){
        var on=c===chip;
        c.setAttribute('aria-checked',on?'true':'false');
        c.tabIndex=on?0:-1;
      });
      if(g==='kind')root.setAttribute('data-kindfilter',filters.kind);
      applyFilters();
      describe(pinnedCell());
      return;
    }
    if(e.target.closest('[data-close]')){P.box.removeAttribute('data-open');return;}
    // A link inside a header cell is a link. Let it be one.
    if(e.target.closest('a'))return;
    var td=e.target.closest('td[data-cell]');
    if(!td)return;
    var rw=G[rows.indexOf(td.parentNode)];
    pin(rows.indexOf(td.parentNode),rw.indexOf(td),{focus:false,open:true});
  });

  // Hover previews the panel and the crosshair without moving the pin.
  // mouseover fires again for every descendant, so a cell is only re-read
  // when the pointer has genuinely crossed into a different one.
  var hovered=null;
  grid.addEventListener('mouseover',function(e){
    var td=e.target.closest('td[data-cell]');
    if(!td||td===hovered)return;
    hovered=td;
    describe(td);
    crosshair(td,false);
  });
  grid.addEventListener('mouseleave',function(){
    hovered=null;
    describe(pinnedCell());
    crosshair(pinnedCell(),false);
  });

  // Arrow keys, only when a cell button has focus. Handling them on the table
  // would swallow the arrow keys of every link in the header row.
  grid.addEventListener('keydown',function(e){
    var btn=e.target.closest('td[data-cell] button');
    if(!btn)return;
    var k=e.key,dr=0,dc=0;
    if(k==='ArrowRight')dc=1;
    else if(k==='ArrowLeft')dc=-1;
    else if(k==='ArrowDown')dr=1;
    else if(k==='ArrowUp')dr=-1;
    else if(k==='Home')dc=-99;
    else if(k==='End')dc=99;
    else if(k==='Enter'||k===' '){e.preventDefault();pin(cur.r,cur.c,{focus:true,open:true});return;}
    else return;
    e.preventDefault();
    var r=cur.r,c=cur.c;
    if(dc===-99){pin(r,0,{focus:true,open:true});return;}
    if(dc===99){pin(r,G[r].length-1,{focus:true,open:true});return;}
    // Step over anything the filters have dimmed: arrowing onto a cell the
    // reader has just filtered away is the filter appearing not to work.
    for(var i=0;i<G.length*G[0].length;i++){
      r+=dr;c+=dc;
      if(r<0||r>=G.length||c<0||c>=G[r].length)return;
      if(!G[r][c].hasAttribute('data-dim')){pin(r,c,{focus:true,open:true});return;}
    }
  });

  function fromHash(){
    if(!location.hash)return null;
    var want=decodeURIComponent(location.hash.slice(1));
    for(var r=0;r<G.length;r++)
      for(var c=0;c<G[r].length;c++)
        if(G[r][c].getAttribute('data-cell')===want)return {r:r,c:c};
    return null;
  }
  document.addEventListener('keydown',function(e){
    if(e.key==='Escape')P.box.removeAttribute('data-open');
  });
  window.addEventListener('hashchange',function(){
    var h=fromHash();
    if(h)pin(h.r,h.c,{focus:false});
  });

  root.setAttribute('data-kindfilter',filters.kind||'all');
  applyFilters();
  var h=fromHash()||{r:0,c:0};
  cur=h;
  describe(pinnedCell());
  crosshair(pinnedCell(),false);
  paintPin();
})();
`
