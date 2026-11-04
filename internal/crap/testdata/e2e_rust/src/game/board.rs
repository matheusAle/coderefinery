pub fn place(ready: bool) -> Option<i32> {
    let v = ready.then_some(1)?;
    Some(v)
}
