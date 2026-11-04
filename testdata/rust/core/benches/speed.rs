// benches/speed.rs: namespace ub_core::benches::speed.
pub fn bench(iterations: u64) -> u64 {
    let mut acc = 0u64;
    for i in 0..iterations {
        acc = acc.wrapping_add(i * 3 + 1);
    }
    acc
}
