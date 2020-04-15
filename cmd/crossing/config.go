package main

import (
	"time"

	"github.com/midbel/toml"
)

func Configure(file string) (Setting, error) {
	var s Setting
	return s, toml.DecodeFile(file, &s)
}

type Area struct {
	Label string

	Lat    float64 `toml:"latitude"`
	Lng    float64 `toml:"longitude"`
	Margin float64

	Night bool

	Starts time.Time `toml:"dtstart"`
	Ends   time.Time `toml:"dtend"`
}

func (a Area) Accept() (Accepter, error) {
	sq, err := NewSquare(a.Lat, a.Lng, a.Margin)
	if err != nil {
		return nil, err
	}
	pd := Period{
		Starts: a.Starts,
		Ends:   a.Ends,
	}
	return NewFilter(a.Label, sq, pd, Eclipse(a.Night)), nil
}

type Setting struct {
	File  string
	List  bool
	Comma bool `toml:"csv"`

	Areas []Area `toml:"area"`
}

func (s Setting) Accept() (Accepter, error) {
	cs := make(multiaccepter, 0, len(s.Areas))
	for _, a := range s.Areas {
		c, err := a.Accept()
		if err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return cs, nil
}
