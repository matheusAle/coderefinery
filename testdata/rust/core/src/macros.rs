//! Macros: nothing inside a token tree counts.
macro_rules! square {
    ($x:expr) => {
        $x * $x
    };
    ($($x:expr),+ $(,)?) => {{
        let mut v = Vec::new();
        $( v.push(square!($x)); )+
        v
    }};
}

macro_rules! maybe [
    ($cond:expr, $then:expr) => { if $cond { $then } else { Default::default() } };
];

macro_rules! nested_rep {
    ($($name:ident : $ty:ty = $val:literal);* ; @tail $t:tt) => ( $( let $name: $ty = $val; )* );
}

pub fn macro_heavy(a: bool, b: Option<u8>) -> u8 {
    println!("{}", if a { 1 } else { 2 });
    assert!(a && a || !a);
    let v = vec![b?, 2, 3];
    debug_assert_eq!(v.len(), 3, "len {}", match v.len() { 3 => "ok", _ => "bad" });
    let s = square!(2u8);
    let m = maybe!(a, 7u8);
    s + m + v[0]
}

pub fn format_args_only(name: &str) -> String {
    format!("hello {name}, {0} {x:>8.3}", 1, x = 2.5)
}

macro_rules! define_getter {
    ($field:ident) => {
        pub fn $field(&self) -> u8 {
            self.$field
        }
    };
}

pub struct Getters {
    width: u8,
}

impl Getters {
    define_getter!(width);

    pub fn area(&self) -> u8 {
        self.width.checked_mul(self.width).unwrap_or(u8::MAX)
    }
}
