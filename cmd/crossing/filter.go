package main

import (
	"fmt"
	"time"
)

type Accepter interface {
	Accept(Point) (bool, string)
}

type filter struct {
	label string
	as    []Accepter
}

func NewFilter(label string, cs ...Accepter) Accepter {
	vs := make([]Accepter, len(cs))
	copy(vs, cs)

	f := filter{
		label: label,
		as:    vs,
	}
	return f
}

func (f filter) Accept(pt Point) (bool, string) {
	for _, c := range f.as {
		ok, _ := c.Accept(pt)
		if !ok {
			return ok, ""
		}
	}
	return true, f.label
}

type Period struct {
	Starts time.Time
	Ends   time.Time
}

func NewPeriod(starts, ends string) (Period, error) {
	var (
		pd  Period
		err error
	)
	if pd.Starts, err = parseTime(starts); err != nil {
		return pd, err
	}
	if pd.Ends, err = parseTime(ends); err != nil {
		return pd, err
	}
	return pd, nil
}

func (p Period) Accept(pt Point) (bool, string) {
	if !p.Starts.IsZero() && pt.When.Before(p.Starts) {
		return false, ""
	}
	return p.Ends.IsZero() || pt.When.Before(p.Ends), ""
}

func (p Period) String() string {
	if p.IsZero() {
		return "crossing time range [,]"
	}
	return fmt.Sprintf("crossing time range [%s,%s]", p.Starts.Format(time.RFC3339), p.Ends.Format(time.RFC3339))
}

func (p Period) IsZero() bool {
	return p.Starts.IsZero() && p.Ends.IsZero()
}

type Square struct {
	East  float64
	West  float64
	North float64
	South float64
}

func NewSquare(lat, lng, margin float64) (Square, error) {
	sq := Square{
		East:  lng - margin,
		West:  lng + margin,
		North: lat + margin,
		South: lat - margin,
	}
	if margin == 0 {
		return sq, fmt.Errorf("zero margin")
	}
	return sq, nil
}

func (s Square) Accept(pt Point) (bool, string) {
	ns := pt.Lat >= s.South && pt.Lat <= s.North
	ew := pt.Lng >= s.East && pt.Lng <= s.West

	return ns && ew, ""
}

func (s Square) String() string {
	return fmt.Sprintf("crossing area: [%.3fS,%.3fN]x[%.3fW,%.3fE]", s.South, s.North, s.West, s.East)
}

type Eclipse bool

func (e Eclipse) Accept(pt Point) (bool, string) {
	ok := bool(e)
	return !ok || pt.Eclipse, ""
}

func (e Eclipse) String() string {
	ok := bool(e)
	if ok {
		return "crossing night only passes"
	}
	return "crossing day and night passes"
}

func parseTime(str string) (time.Time, error) {
	var (
		when time.Time
		err  error
	)
	if str == "" {
		return when, err
	}
	for _, pat := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		when, err = time.Parse(pat, str)
		if err == nil {
			break
		}
	}
	return when, err
}
