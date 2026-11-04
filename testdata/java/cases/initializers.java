class A { static { if (a) {} } void m() { new Thread() { public void run() {} }; try (var x = r()) {} catch (A | B e) {} finally {} } }
@interface Ann { String v() default ""; }
