#!/bin/sh
# usage: deck17-run.sh <w> <h> : reach / probe / bottom / widths / frames on /, /agents, /math (session $AGENT_BROWSER_SESSION)
cd /Users/matheusale/workspaces/ub-mode/site
agent-browser set viewport "$1" "$2" >/dev/null
for r in "" agents math; do
  agent-browser open "http://localhost:5173/$r?r=$$" >/dev/null; agent-browser wait 2200 >/dev/null
  agent-browser eval "$(cat .tools/deck17.js)" >/dev/null
  echo "== /$r $1x$2 mode=$(agent-browser eval "document.documentElement.dataset.mode+' '+document.querySelector('main').className+' coarse='+matchMedia('(pointer: coarse)').matches" | tr -d '"')"
  agent-browser eval "(async()=>{const r=JSON.parse(await D17.reach());return 'reach all='+r.all+' cards='+r.out.length+' tall='+r.out.filter(o=>o.tall).length+' bad='+JSON.stringify(r.out.filter(o=>!o.ok).map(o=>[o.card,o.topAt,o.bottomAt,o.holdEnd]))})()" | tr -d '"'
  agent-browser eval "D17.probe(60)" | tr -d '\\'
  agent-browser eval "(async()=>{const b=JSON.parse(await D17.bottom());return 'bottom foreign='+b.map(x=>x.foreign.length).join(',')+' visibleEarlier='+b.map(x=>x.visibleEarlier.length).join(',')+' last@max='+b[4].lastTop+' footer@max='+b[4].footerTop})()" | tr -d '"'
  agent-browser eval "D17.widths()" | tr -d '\\'
  agent-browser eval "D17.startFrames(30)" >/dev/null
  i=0; while [ $i -lt 60 ]; do sleep 2; v=$(agent-browser eval "D17.result" | tr -d '\\'); [ "$v" != "null" ] && break; i=$((i+1)); done; echo "frames $v"
done
