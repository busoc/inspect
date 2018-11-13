package main

import (
	"fmt"

	"github.com/busoc/celest"
)

type rect struct {
	North float64
	South float64
	West  float64
	East  float64
}

func (r *rect) Contains(p celest.Point) bool {
	return (p.Lat > r.South && p.Lat < r.North) && (p.Lon > r.West && p.Lon < r.East)
}

func (r *rect) Set(s string) error {
	_, err := fmt.Sscanf(s, "%f:%f:%f:%f", &r.North, &r.East, &r.South, &r.West)
	return err
}

func (r *rect) String() string {
	return fmt.Sprintf("rect(%.2fN:%.2fE:%.2fS:%.2fW)", r.North, r.East, r.South, r.West)
}
