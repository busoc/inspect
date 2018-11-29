package main

import (
	"fmt"

	"github.com/busoc/celest"
)

var SAA = rect{
	North: SAALatMax,
	South: SAALatMin,
	East:  SAALonMax,
	West:  SAALonMin,
}

type rect struct {
	North float64
	South float64
	West  float64
	East  float64
	Syst  string
}

func (r *rect) Contains(p celest.Point) bool {
	x := transform(&p, r.Syst)
	return (x.Lat > r.South && x.Lat < r.North) && (x.Lon > r.West && x.Lon < r.East)
}

func (r *rect) Set(s string) error {
	_, err := fmt.Sscanf(s, "%f:%f:%f:%f", &r.North, &r.East, &r.South, &r.West)
	return err
}

func (r *rect) String() string {
	return fmt.Sprintf("rect(%.2fN:%.2fE:%.2fS:%.2fW)", r.North, r.East, r.South, r.West)
}
