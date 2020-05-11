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

func (s Setting) Paths() ([]Path, error) {
	var (
		paths []Path
		files = []string{s.File}
	)
	for _, a := range s.Areas {
		accept, err := a.Accept()
		if err != nil {
			return nil, err
		}
		ps, err := ReadPaths(files, accept)
		if err != nil {
			return nil, err
		}
		paths = append(paths, ps...)
	}
	// sort.Slice(paths)
	return paths, nil
}
