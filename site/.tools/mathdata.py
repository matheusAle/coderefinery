"""Extract the /math explainer states from the regenerated reference into JSON (copy verbatim)."""
import re, json, sys, html as H
from html.parser import HTMLParser

SRC = open(sys.argv[1]).read()

class Node:
    def __init__(s, tag, attrs, parent):
        s.tag = tag; s.a = dict(attrs); s.kids = []; s.parent = parent; s.text = ''
        st = {}
        for part in s.a.get('style', '').split(';'):
            if ':' in part:
                k, v = part.split(':', 1); st[k.strip()] = ' '.join(v.split())
        s.st = st
        s.name = s.a.get('data-pencil-name')

class P(HTMLParser):
    def __init__(s):
        super().__init__(); s.root = Node('root', [], None); s.cur = s.root
    def handle_starttag(s, tag, attrs):
        n = Node(tag, attrs, s.cur); s.cur.kids.append(n)
        if tag != 'br': s.cur = n
    def handle_endtag(s, tag):
        if tag != 'br' and s.cur.parent: s.cur = s.cur.parent
    def handle_data(s, d):
        t = ' '.join(H.unescape(d).split())
        if t:
            n = Node('#text', [], s.cur); n.text = t; s.cur.kids.append(n)

p = P(); p.feed(SRC)

def walk(n):
    yield n
    for k in n.kids: yield from walk(k)

def color(n):
    while n:
        if 'color' in n.st: return n.st['color']
        n = n.parent
    return ''

def texts(n):
    return [k for k in walk(n) if k.tag == '#text']

def find(n, name):
    for k in walk(n):
        if k.name == name: return k
    return None

def findall(n, pred):
    return [k for k in walk(n) if pred(k)]

TONE = {'#FF6A1A': 'molten', '#ECEDEF': 'chalk', '#8B8F99': 'ash', '#FF4D4D': 'slag', '#3DDC84': 'ingot', '#FFB020': 'ember',
        '#7CB8FF': 'kw', '#8FD9A8': 'str', '#FFC95C': 'num', '#C9A0FF': 'dec'}
TINT = {'#FF6A1A26': 'molten', '#3DDC8422': 'ingot', '#FFB02022': 'ember', '#FF4D4D22': 'slag'}

states = []
for n in walk(p.root):
    if n.name and re.match(r'M[1-4]\.\d ·', n.name):
        sid = n.name.split(' ')[0]
        formula = [{'text': t.text, 'molten': color(t).upper() == '#FF6A1A'} for t in texts(find(n, 'Formula'))]
        code = find(n, 'Code')
        fb = [t.text for t in texts(find(code, 'Filebar'))]
        lines = []
        for L in findall(code, lambda k: k.name and re.fullmatch(r'L\d+', k.name)):
            c = find(L, 'code')
            toks = []
            for t in walk(c):
                if t.tag == '#text': toks.append([t.text, TONE.get(color(t).upper(), 'chalk')])
            # indentation: count empty leading <#t> spans
            lead = 0
            for k in c.kids:
                if k.name == 't' and not texts(k): lead += 1
                else: break
            note = find(L, 'Note')
            ntxt = ' '.join(t.text for t in texts(note)) if note else ''
            tint = TINT.get(L.st.get('background-color', '').upper(), '')
            strike = any(k.st.get('text-decoration', '').startswith('line-through') for k in walk(L))
            lines.append({'n': int(L.name[1:]), 'indent': lead, 'tokens': toks, 'tint': tint, 'note': ntxt,
                          'noteTone': TONE.get(color(texts(note)[0]).upper(), '') if note else '', 'strike': strike})
        card = find(n, 'Card')
        ct = texts(card)
        title, body = ct[0].text, ct[1].text
        heur = find(card, 'Heuristic')
        rule = ' '.join(t.text for t in texts(heur)[1:])
        vis = find(card, 'Visual')
        status = [t.text for t in texts(find(n, 'Statusbar'))]
        states.append({'id': sid, 'formula': formula, 'file': fb, 'lines': lines, 'title': title, 'body': body, 'rule': rule,
                       'visual': bool(vis), 'status': status})
json.dump(states, sys.stdout, ensure_ascii=False, indent=1)
