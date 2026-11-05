"""Dump a Pencil HTML export as an indented outline: frames with key layout, text nodes with font and colour."""
import sys, re, html
from html.parser import HTMLParser

def sty(s):
    d = {}
    for part in s.split(';'):
        if ':' in part:
            k, v = part.split(':', 1)
            d[k.strip()] = ' '.join(v.split())
    return d

class P(HTMLParser):
    def __init__(self):
        super().__init__(); self.stack = []; self.out = []
    def handle_starttag(self, tag, attrs):
        a = dict(attrs)
        if tag in ('br',):
            self.out.append('  ' * len(self.stack) + '<br>'); return
        st = sty(a.get('style', ''))
        name = a.get('data-pencil-name')
        self.stack.append((tag, st))
        if name:
            bits = []
            for k, lab in (('background-color', 'bg'), ('width', 'w'), ('height', 'h'), ('gap', 'gap'), ('padding', 'pad'), ('flex-direction', '')):
                if k in st: bits.append(f'{lab}{st[k]}')
            self.out.append('  ' * (len(self.stack) - 1) + f'<#{name} | ' + ' | '.join(bits) + '>')
    def handle_endtag(self, tag):
        if tag == 'br': return
        if self.stack: self.stack.pop()
    def handle_data(self, data):
        t = ' '.join(html.unescape(data).split())
        if not t: return
        st = {}
        for _, s in self.stack: st.update(s)
        fam = st.get('font-family', '').split(',')[0].strip('"\' ')
        self.out.append('  ' * len(self.stack) + f'"{t}"  [{fam} {st.get("font-size","")}/{st.get("font-weight","")} {st.get("color","")}{" bg"+st["background-color"] if "background-color" in self.stack[-1][1] else ""}]')

p = P(); p.feed(open(sys.argv[1]).read())
print('\n'.join(p.out))
