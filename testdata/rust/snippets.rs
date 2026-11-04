// Upstream crapper test snippets (tests/test_rust.py, test_edges.py,
// test_languages.py, test_cli.py) and spec-crapper §8.5 observed cases,
// gathered for parser parity. Outside any [package]: crate::snippets.

pub fn choose(x: i32) -> i32 {
    if x > 0 && x < 10 {
        return 1;
    }
    match x {
        1 => 1,
        2 => 2,
        _ => 0,
    }
}

impl Sample {
    pub fn run(&self) -> i32 {
        if self.ok { 1 } else { 0 }
    }
}

impl Trait for Sample {
    fn draw(&mut self) {}
}

mod tests {
    fn hidden() {
        if true {}
    }
}

fn outer() {
    fn inner() {
        if true {}
    }
}

impl Trait for Vec<crate::Sample> {
    fn draw(&self) {}
}

pub fn open() {}
pub fn main_ok() {}
pub fn loose() {}
pub fn bare() {}

pub fn place() -> i32 { if ready { 1 } else { 0 } }

trait Foo {
    fn draw(&self) {}
}

pub fn open_ok(ok: bool) -> i32 { if ok { 1 } else { 0 } }

fn f1(a: bool) { println!("{}", if a { 1 } else { 2 }); assert!(a && b); vec![x?]; }
fn f2() { if let Some(x) = y && x > 0 && let Ok(z) = w { } while let Some(v) = it.next() {} }
fn f3() { let c = |x| if x { 1 } else { 2 }; let d = async { a? }; match x { 1 | 2 if g => 1, _ => 2 } }
fn f4() { struct S; impl S { fn m(&self) { if a {} } } }
mod a { mod tests { fn t() {} } mod b { fn g() { if a {} else if b {} else {} } } }
#[cfg(test)]
mod test { fn h() {} }
impl<T: Clone> fmt::Display for Wrapper<T> where T: X { fn fmt(&self) {} }
impl Foo for &Bar { fn a() {} }
impl dyn Tr { fn b() {} }
impl<T> Foo<T> { fn c() {} }
impl crate::a::B { fn d() {} }
fn f5() { if (a && b) && let Some(x) = y {} while let P = a && b || c {} if let P = (a && b) {} }
fn f6() { let a = || a && b; let b = move || x?; let r = &&x; 'outer: loop { break 'outer; } let t = async { a? }; }
fn f7<T: ?Sized + for<'a> Fn(&'a T)>(x: &&T) -> Box<dyn Iterator<Item = &T> + '_> { todo!() }
