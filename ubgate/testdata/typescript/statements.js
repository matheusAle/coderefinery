for (let i = 0; i < n; i++) {}
for (;;) ;
for (i = 0, j = 1; ; ) {}
for (var x in o) {}
for (const [a, b] of o) {}
for await (const y of xs) {}
for (x.y in o) {}
for (var k = 1 in o) {}
while (a) b();
do { } while (a)
do x(); while (b);
if (a) b(); else if (c) {} else d;
switch (a) { case 1: case 2: f(); break; default: g() }
try {} catch {} finally {}
try {} catch (e) {}
try {} catch ({a}) {}
lbl: for (;;) { break lbl; continue lbl; }
throw new Error("x");
return;
debugger;
with (a) {}
;
