//! board: nested module ub_core::game::board.
use super::Cell;
use std::collections::{HashMap, HashSet as Set};

pub struct Board {
    pub cells: Vec<Cell>,
    seen: Set<usize>,
    index: HashMap<String, usize>,
}

pub fn place() -> i32 {
    if ready() { 1 } else { 0 }
}

fn ready() -> bool {
    false
}

impl Board {
    pub fn new(size: usize) -> Self {
        Board { cells: vec![Cell::Empty; size], seen: Set::new(), index: HashMap::new() }
    }

    pub fn mark(&mut self, at: usize, by: u8) -> Result<(), String> {
        let cell = self.cells.get_mut(at).ok_or_else(|| format!("no cell {at}"))?;
        if let Cell::Empty = cell {
            *cell = Cell::Marked { by, turn: self.seen.len() as u32 };
            self.seen.insert(at);
            Ok(())
        } else if let Cell::Taken(owner) = cell {
            Err(format!("taken by {owner}"))
        } else {
            Err(String::from("marked"))
        }
    }

    pub fn first_free(&self) -> Option<usize> {
        let mut i = 0;
        'scan: loop {
            while i < self.cells.len() {
                if self.cells[i] == Cell::Empty {
                    break 'scan Some(i);
                }
                i += 1;
            }
            break None;
        }
    }

    pub fn lookup(&self, key: &str) -> Option<&usize> {
        if let Some(found) = self.index.get(key) && *found < self.cells.len() {
            return Some(found);
        }
        while let Some(alt) = key.strip_prefix('_') && !alt.is_empty() || key.len() > 64 {
            return self.index.get(alt);
        }
        None
    }

    pub fn counts(&self) -> (usize, usize) {
        let taken = self.cells.iter().filter(|c| matches!(c, Cell::Taken(_))).count();
        let marked = self
            .cells
            .iter()
            .filter(|c| match c {
                Cell::Marked { turn, .. } => *turn > 0 && *turn < 100,
                _ => false,
            })
            .count();
        (taken, marked)
    }
}
