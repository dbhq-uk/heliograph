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

/* The page breaks its measure. A grid seven columns wide inside 80ch is a
   horizontal scrollbar on a desktop, which is the one place it should not be. */
.docs-shell--wide{max-width:118rem}
.docs-shell--wide main{max-width:none}
.mxa{margin:1.6rem 0 2.2rem}

/* -- the control bar -- */
.mxa-bar{display:flex;flex-wrap:wrap;gap:1.1rem 1.6rem;align-items:flex-end;
  padding:.9rem 1rem;border:1px solid var(--ridge);border-radius:12px;background:var(--dusk)}
.mxa-fl{display:block;font-size:.7rem;letter-spacing:.09em;text-transform:uppercase;
  color:var(--ink-3);margin-bottom:.4rem}
.mxa-chips{display:flex;flex-wrap:wrap;gap:.3rem}
.mxa-chip{border:1px solid var(--ridge);background:none;color:var(--ink-2);font:inherit;
  font-size:.82rem;border-radius:999px;padding:.22rem .7rem;cursor:pointer;
  transition:border-color .15s,color .15s,background .15s}
.mxa-chip:hover{border-color:var(--brass);color:var(--ink)}
.mxa-chip:focus-visible{outline:2px solid var(--gold);outline-offset:2px}
.mxa-chip[aria-checked="true"]{background:var(--gold);border-color:var(--gold);color:#06213A;font-weight:600}
.mxa-count{margin:0 0 .1rem auto;font-size:.85rem;color:var(--ink-3);white-space:nowrap}
.mxa-count b{color:var(--flash);font-size:1.5rem;font-weight:600;line-height:1;
  font-variant-numeric:tabular-nums;margin-right:.15rem}

/* -- the body: grid beside panel, stacked on a narrow screen -- */
.mxa-body{display:grid;grid-template-columns:minmax(0,1fr) 20rem;gap:1.1rem;margin-top:1.1rem;align-items:start}
@media(max-width:70rem){
  .mxa-body{grid-template-columns:1fr}
  /* Stacked, it is no longer a side panel, and a sticky full-width block
     under the grid follows the reader down the page covering it. */
  .mxa-panel{position:static}
}
.mxa-gridwrap{overflow-x:auto;border:1px solid var(--ridge);border-radius:12px;background:var(--dusk)}

/* min-width is what makes the wrapper's overflow-x real. Without it,
   table-layout:fixed honours width:100% at any size and squeezes seven data
   columns into whatever is left beside an 11.5rem sticky label column. */
.mxa-grid{border-collapse:separate;border-spacing:0;width:100%;min-width:40rem;margin:0;
  font-size:.9rem;table-layout:fixed}
.mxa-grid td,.mxa-grid th{border:0;padding:0;vertical-align:middle}
.mxa-corner{width:11.5rem;min-width:11.5rem;padding:.5rem .7rem;line-height:1.15}
.mxa-corner span{display:block;font-size:.62rem;letter-spacing:.08em;text-transform:uppercase;color:var(--ink-3)}
.mxa-corner span:first-child{text-align:left}
.mxa-corner span:last-child{text-align:right;color:var(--brass)}

.mxa-grid thead th{padding:.5rem .25rem;text-align:center;font-size:.74rem;font-weight:600;
  letter-spacing:.02em;text-transform:none;color:var(--ink-2);border-bottom:1px solid var(--ridge);
  transition:color .15s,background .15s}
.mxa-grid thead th a{color:inherit;text-decoration:none}
.mxa-grid thead th a:hover{text-decoration:underline}
.mxa-grid thead th em{display:block;font-style:normal;font-size:.58rem;letter-spacing:.08em;
  text-transform:uppercase;color:var(--ink-3);margin-top:.12rem}

/* Sticky, because the grid scrolls sideways on a phone and a row of dots
   with the label scrolled off is seven facts about nothing. */
.mxa-grid tbody th{text-align:left;padding:.42rem .7rem;font-size:.82rem;font-weight:500;
  color:var(--ink-2);text-transform:none;letter-spacing:0;border-right:1px solid var(--ridge);
  transition:color .15s,background .15s;position:sticky;left:0;z-index:2;background:var(--dusk)}
.mxa-corner{position:sticky;left:0;z-index:3;background:var(--dusk)}
.mxa-grid tbody th a{color:inherit;text-decoration:none}
.mxa-grid tbody th a:hover{text-decoration:underline}
.mxa-grid tbody th em{display:block;font-style:normal;font-size:.62rem;color:var(--ink-3)}
.mxa-grid tbody tr:nth-child(even) td{background:rgba(255,255,255,.012)}

/* -- the cells -- */
.mxa-grid tbody td{text-align:center}
.mxa-grid tbody td button{display:block;width:100%;border:0;background:none;padding:.42rem .2rem;
  cursor:pointer;color:inherit}
/* A visible focus ring is the whole keyboard story: the pin moves focus to a
   cell button, and suppressing the outline with nothing in its place made
   that move invisible. */
.mxa-grid tbody td button:focus{outline:none}
.mxa-grid tbody td button:focus-visible{outline:2px solid var(--flash);outline-offset:-2px;border-radius:6px}
/* EVERY STATE HAS ITS OWN SHAPE as well as its own hue. Proven and works
   were both solid rounded squares and differed by colour alone, which is the
   distinction a colour-blind reader loses first - and there are five states
   here, so hue on its own was never going to carry it. */
.mxa-dot{display:inline-block;width:1.05rem;height:1.05rem;border-radius:4px;
  background:var(--mx-none);transition:transform .18s var(--ease),box-shadow .18s}
[data-state="proven"] .mxa-dot{background:var(--mx-proven);border-radius:4px}      /* filled square */
[data-state="works"] .mxa-dot{background:var(--mx-works);border-radius:50%}        /* filled circle */
[data-state="needs"] .mxa-dot{background:none;border:2px solid var(--mx-needs);
  border-radius:50%}                                                              /* hollow circle */
[data-state="untested"] .mxa-dot{background:none;border:2px dashed var(--mx-untested);
  border-radius:4px}                                                              /* dashed square */
[data-state="none"] .mxa-dot{background:none;position:relative;opacity:.5}
[data-state="none"] .mxa-dot:before,[data-state="none"] .mxa-dot:after{
  content:"";position:absolute;inset:45% 12%;background:var(--slate);border-radius:1px}
[data-state="none"] .mxa-dot:before{transform:rotate(45deg)}
[data-state="none"] .mxa-dot:after{transform:rotate(-45deg)}
.mxa-grid tbody td:hover .mxa-dot{transform:scale(1.22)}

/* -- the crosshair, and the flash -- */
.mxa-grid tbody tr[data-lit] th,.mxa-grid thead th[data-lit]{color:var(--flash);background:rgba(123,167,212,.09)}
.mxa-grid td[data-lit]{background:rgba(123,167,212,.06)}
.mxa-grid td[data-on] .mxa-dot{box-shadow:0 0 0 3px rgba(230,241,251,.22),0 0 16px 2px currentColor;transform:scale(1.3)}
.mxa-grid td[data-on]{background:rgba(123,167,212,.14);box-shadow:inset 0 0 0 1px var(--gold)}

/* The heliograph is a mirror flashing light across a valley. A selection
   sweeps the row it lit, which is the one piece of motion on the page. */
@keyframes mx-beam{from{background-position:-40% 0}to{background-position:140% 0}}
.mxa-grid tbody tr[data-lit]{background-image:linear-gradient(90deg,
  transparent 0%,rgba(230,241,251,.10) 45%,rgba(230,241,251,.16) 50%,rgba(230,241,251,.10) 55%,transparent 100%);
  background-size:60% 100%;background-repeat:no-repeat;animation:mx-beam .55s var(--ease) 1}
@media(prefers-reduced-motion:reduce){
  .mxa-grid tbody tr[data-lit]{animation:none;background-image:none}
  .mxa-dot{transition:none}
}

/* -- the panel -- */
.mxa-panel{border:1px solid var(--ridge);border-radius:12px;background:var(--dusk);
  padding:1rem 1.1rem 1.1rem;position:sticky;top:1rem}
.mxa-state{margin:0 0 .45rem;font-size:.68rem;letter-spacing:.1em;text-transform:uppercase;
  font-weight:700;display:inline-block;padding:.16rem .5rem;border-radius:5px;color:#06213A}
.mxa-state[data-state="proven"]{background:var(--mx-proven)}
.mxa-state[data-state="works"]{background:var(--mx-works)}
.mxa-state[data-state="needs"]{background:var(--mx-needs)}
.mxa-state[data-state="untested"]{background:var(--mx-untested)}
.mxa-state[data-state="none"]{background:var(--slate);color:var(--ink-2)}
.mxa-panel h3{margin:0 0 .5rem;font-size:1.02rem;line-height:1.3;color:var(--flash)}
.mxa-panel h3 span{color:var(--ink-3);font-weight:400}
.mxa-panel p[data-panel-note]{margin:0 0 .8rem;font-size:.88rem;color:var(--ink-2);line-height:1.5}
.mxa-meta{margin:0 0 .8rem;display:grid;gap:.3rem;font-size:.78rem}
.mxa-meta div{display:flex;gap:.5rem;justify-content:space-between;border-bottom:1px solid var(--ridge);padding-bottom:.25rem}
.mxa-meta dt{color:var(--ink-3);margin:0}
.mxa-meta dd{margin:0;color:var(--ink-2);text-align:right}
.mxa-links{margin:0 0 .5rem;display:flex;gap:.8rem;flex-wrap:wrap;font-size:.82rem}
.mxa-hint{margin:0;font-size:.74rem;color:var(--ink-3)}

/* -- legend and the folded tables -- */
.mxa-legend{list-style:none;display:flex;flex-wrap:wrap;gap:.35rem 1.1rem;margin:.9rem 0 0;padding:0;
  font-size:.78rem;color:var(--ink-3)}
.mxa-legend li{display:flex;align-items:center;gap:.4rem}
.mxa-legend .mxa-dot{width:.8rem;height:.8rem;border-radius:4px}
.mxa-tables{margin:1rem 0;border:1px solid var(--ridge);border-radius:10px;background:var(--dusk)}
.mxa-tables summary{cursor:pointer;padding:.6rem .9rem;font-size:.88rem;color:var(--ink-2)}
.mxa-tables summary:hover{color:var(--flash)}
.mxa-tables[open] summary{border-bottom:1px solid var(--ridge)}
.mxa-tables .tw{padding:0 .9rem .4rem}

/* A dimmed column or row is a filter answering, and it must still be readable
   rather than invisible: the reader is deciding whether their case is here. */
.mxa-grid [data-dim]{opacity:.22}
.mxa-grid [data-dim] .mxa-dot{filter:grayscale(1)}
.mxa-panel[data-hidden]{border-style:dashed;opacity:.72}
.mxa-panel[data-hidden]:before{content:"Filtered out of the grid above";display:block;
  font-size:.68rem;letter-spacing:.08em;text-transform:uppercase;color:var(--mx-needs);margin-bottom:.5rem}
.mxa-empty{margin:.9rem 0 0;padding:.7rem .9rem;border:1px dashed var(--mx-needs);border-radius:8px;
  color:var(--ink-2);font-size:.88rem}
.mxa-empty[hidden]{display:none}
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
      applyFilters();
      describe(pinnedCell());
      return;
    }
    // A link inside a header cell is a link. Let it be one.
    if(e.target.closest('a'))return;
    var td=e.target.closest('td[data-cell]');
    if(!td)return;
    var rw=G[rows.indexOf(td.parentNode)];
    pin(rows.indexOf(td.parentNode),rw.indexOf(td),{focus:false});
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
    else if(k==='Enter'||k===' '){e.preventDefault();pin(cur.r,cur.c,{focus:true});return;}
    else return;
    e.preventDefault();
    var r=cur.r,c=cur.c;
    if(dc===-99){pin(r,0,{focus:true});return;}
    if(dc===99){pin(r,G[r].length-1,{focus:true});return;}
    // Step over anything the filters have dimmed: arrowing onto a cell the
    // reader has just filtered away is the filter appearing not to work.
    for(var i=0;i<G.length*G[0].length;i++){
      r+=dr;c+=dc;
      if(r<0||r>=G.length||c<0||c>=G[r].length)return;
      if(!G[r][c].hasAttribute('data-dim')){pin(r,c,{focus:true});return;}
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
  window.addEventListener('hashchange',function(){
    var h=fromHash();
    if(h)pin(h.r,h.c,{focus:false});
  });

  applyFilters();
  var h=fromHash()||{r:0,c:0};
  cur=h;
  describe(pinnedCell());
  crosshair(pinnedCell(),false);
  paintPin();
})();
`
