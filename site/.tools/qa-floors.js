(async()=>{const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const H=document.documentElement.scrollHeight;for(let y=0;y<H;y+=500){scrollTo({top:y,behavior:'instant'});await sleep(60);}
await sleep(1500);scrollTo({top:0,behavior:'instant'});await sleep(300);
const monoFam=getComputedStyle(document.querySelector('.kicker, .mono')).fontFamily;
const isMono=cs=>cs.fontFamily===monoFam||/mono/i.test(cs.fontFamily);
const sel=e=>e.tagName.toLowerCase()+(e.id?'#'+e.id:'')+'.'+String(e.className).split(' ').filter(Boolean).slice(0,2).join('.');
const vis=e=>{const r=e.getBoundingClientRect();return r.width>0&&r.height>0&&getComputedStyle(e).visibility!=='hidden'};
const out={pad:[],cell:[],head:[],body:[],mono:[],tap:[]};
// surfaces
const surf=[...document.querySelectorAll('.box, .pane, .term, .hint, .statusbar, .pain-col, .ex-card, .tstats > *, .bignums > *, .pnums > *, .stat, .tile, .cell, .gate-cell, .a-cell, .vtable, .loop-step, blockquote')].filter(vis);
const seenS=new Set();
surf.forEach(e=>{const r=e.getBoundingClientRect();const tr=[];const w=document.createTreeWalker(e,NodeFilter.SHOW_TEXT);let n;while(n=w.nextNode()){if(!n.textContent.trim())continue;const pe=n.parentElement;if(pe.closest('.sr-only, svg')||getComputedStyle(pe).visibility==='hidden')continue;const rg=document.createRange();rg.selectNodeContents(n);[...rg.getClientRects()].forEach(q=>{if(q.width>0&&q.height>0&&q.top>=r.top-1&&q.bottom<=r.bottom+1)tr.push(q)});}
 if(!tr.length)return;const L=Math.min(...tr.map(q=>q.left))-r.left,T=Math.min(...tr.map(q=>q.top))-r.top,B=r.bottom-Math.max(...tr.map(q=>q.bottom));const m=Math.min(L,T,B);const narrow=r.width<180;const sc=[...e.querySelectorAll('*')].some(x=>getComputedStyle(x).overflowX==='auto'&&x.scrollWidth>x.clientWidth)||getComputedStyle(e).overflowX==='auto';
 out.pad.push({k:sel(e),w:Math.round(r.width),L:+L.toFixed(1),T:+T.toFixed(1),B:+B.toFixed(1),floor:narrow?16:20,ok:m>=(narrow?16:20)-0.6});});
document.querySelectorAll('th, td, [role=cell], [role=columnheader], [role=rowheader]').forEach(e=>{if(!vis(e))return;const cs=getComputedStyle(e);out.cell.push({k:sel(e)+' '+e.closest('table,[class]')?.className.slice(0,14),py:Math.min(parseFloat(cs.paddingTop),parseFloat(cs.paddingBottom)),px:Math.min(parseFloat(cs.paddingLeft),parseFloat(cs.paddingRight)),ok:Math.min(parseFloat(cs.paddingTop),parseFloat(cs.paddingBottom))>=12&&Math.min(parseFloat(cs.paddingLeft),parseFloat(cs.paddingRight))>=14});});
document.querySelectorAll('h1,h2,.h2,.display.h1').forEach(e=>{if(!vis(e)||e.classList.contains('sr-only'))return;const cs=getComputedStyle(e);const fs=parseFloat(cs.fontSize),lh=parseFloat(cs.lineHeight)/fs;out.head.push({k:sel(e),fs,lh:+lh.toFixed(3),ok:fs>=28&&Math.abs(lh-1.25)<0.01});});
document.querySelectorAll('body *').forEach(e=>{if(!vis(e)||e.closest('.stream, svg, .sr-only, canvas, .odo-sr'))return;
 const own=[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim().length>1);if(!own)return;const cs=getComputedStyle(e);const fs=parseFloat(cs.fontSize);
 if(isMono(cs)){if(fs<12.5-0.01)out.mono.push({k:sel(e),fs,t:e.textContent.trim().slice(0,30)});}
 else if(e.matches('p,li,blockquote,.lede,span,div,cite')&&fs<15-0.01&&!e.closest('.kicker,.statusbar,.pane-bar'))out.body.push({k:sel(e),fs,t:e.textContent.trim().slice(0,30)});});
document.querySelectorAll('a, button, [role=button], input, summary').forEach(e=>{if(!vis(e))return;const r=e.getBoundingClientRect();
const cx=r.left+r.width/2;const cy=r.top+r.height/2;
 // effective hit area: scroll into view then probe a 44x44 box
 e.scrollIntoView({block:'center',behavior:'instant'});const r2=e.getClientRects()[0];const cx2=Math.min(innerWidth-1,Math.max(0,r2.left+r2.width/2)),cy2=r2.top+r2.height/2;
 const hit=(x,y)=>{const t=document.elementFromPoint(x,y);return t&&(t===e||e.contains(t))};
 let up=0,dn=0;for(let d=1;d<=30;d++){if(hit(cx2,cy2-d))up=d;else break;}for(let d=1;d<=30;d++){if(hit(cx2,cy2+d))dn=d;else break;}
 let lf=0,rt=0;for(let d=1;d<=30;d++){if(hit(cx2-d,cy2))lf=d;else break;}for(let d=1;d<=30;d++){if(hit(cx2+d,cy2))rt=d;else break;}
 const hh=up+dn+1,ww=lf+rt+1;out.tap.push({k:sel(e)+' "'+e.textContent.trim().slice(0,22)+'"',rect:Math.round(r.width)+'x'+Math.round(r.height),hit:Math.min(ww,61)+'x'+Math.min(hh,61),ok:hh>=44&&(ww>=44||r.width>=44)});});
return JSON.stringify(out);})()
