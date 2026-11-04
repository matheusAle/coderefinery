//! game: a mod.rs module (namespace ub_core::game).
pub mod board;

use std::fmt;

#[derive(Debug, Clone, PartialEq)]
pub enum Cell {
    Empty,
    Taken(u8),
    Marked { by: u8, turn: u32 },
}

pub struct Wrapper<T>(pub T);

impl<T: fmt::Debug> fmt::Display for Wrapper<T>
where
    T: Clone,
{
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{:?}", self.0)
    }
}

impl<T> Wrapper<T> {
    pub fn get(&self) -> &T {
        &self.0
    }
}

pub trait Shape {
    fn area(&self) -> f64;
}

impl Shape for &Cell {
    fn area(&self) -> f64 {
        match **self {
            Cell::Empty => 0.0,
            Cell::Taken(n) if n > 3 => 1.5,
            Cell::Taken(_) => 1.0,
            Cell::Marked { by, .. } => by as f64,
        }
    }
}

impl dyn Shape {
    pub fn doubled(&self) -> f64 {
        self.area() * 2.0
    }
}

impl crate::game::board::Board {
    pub fn scoped_impl(&self) -> usize {
        self.cells.len()
    }
}

impl<'a> Iterator for Wrapper<std::slice::Iter<'a, Cell>> {
    type Item = &'a Cell;

    fn next(&mut self) -> Option<Self::Item> {
        self.0.next()
    }
}
