// Package geometry has methods on receivers and a select loop.
package geometry

import "time"

// Rect is an axis-aligned rectangle.
type Rect struct {
	W, H float64
}

// Area of the rectangle; zero for invalid sizes.
func (r Rect) Area() float64 {
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	return r.W * r.H
}

// Scale grows the rectangle in place.
func (r *Rect) Scale(f float64) {
	if f == 1 {
		return
	}
	r.W = r.W * f
	r.H = r.H * f
}

func (r Rect) square() bool {
	return r.W == r.H && r.W != 0
}

// Collect drains values until done or timeout.
func Collect(values <-chan int, done <-chan struct{}, limit int) []int {
	var out []int
	for len(out) < limit {
		select {
		case v, ok := <-values:
			if !ok {
				return out
			}
			out = append(out, v)
		case <-done:
			return out
		case <-time.After(time.Second):
			return out
		}
	}
	return out
}
