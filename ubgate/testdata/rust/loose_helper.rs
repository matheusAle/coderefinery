// Outside any [package]: crate::loose_helper.
pub fn loose(a: bool, b: bool) -> bool {
    a && b || !a
}
