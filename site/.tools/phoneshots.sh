#!/bin/sh
# usage: phoneshots.sh <route> <tag> : scroll each card (and each stacked dive/explainer state) to the top and screenshot
route="$1"; tag="$2"; S=/Users/matheusale/workspaces/ub-mode/site/.shots
agent-browser open "http://localhost:5173/$route" >/dev/null; agent-browser wait 1500 >/dev/null
n=$(agent-browser eval "(()=>{const t=[...document.querySelectorAll('main > .hold, main > footer, .dive.stacked > .dive-stage, .explainer.stacked > .ex-stage')];window.__targets=t;return t.length})()" | tr -d '"')
i=0
while [ $i -lt $n ]; do
  agent-browser eval "(async()=>{const t=window.__targets[$i];const H=document.documentElement.scrollHeight;const y=Math.round(t.getBoundingClientRect().top+scrollY);for(let yy=scrollY;Math.abs(yy-y)>1;yy+=Math.sign(y-yy)*Math.min(300,Math.abs(y-yy))){scrollTo({top:yy,behavior:'instant'});await new Promise(r=>setTimeout(r,20))}scrollTo({top:y,behavior:'instant'});await new Promise(r=>setTimeout(r,900));return $i+' '+(t.querySelector('.kicker,.dive-kicker')?.textContent.trim().slice(0,30)??t.className.split(' ')[0])+' h='+Math.round(t.getBoundingClientRect().height)+' sw='+document.documentElement.scrollWidth+'/'+innerWidth})()" | tr -d '"'
  agent-browser screenshot "$S/$tag-$i.png" >/dev/null
  i=$((i+1))
done
