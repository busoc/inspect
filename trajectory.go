package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	// "strings"
	"time"
)

const (
	emptyLen = 24
	tleLen   = 69
	tleRows  = 2
)

type MissingRowError int

func (e MissingRowError) Error() string {
	return fmt.Sprintf("missing row#%d", int(e)+1)
}

type InvalidLenError int

func (e InvalidLenError) Error() string {
	return fmt.Sprintf("invalid row length %d (%d)", int(e), tleLen)
}

type Trajectory struct {
	elements []*Element
}

func Open(files []string, id int) (*Trajectory, error) {
	var t Trajectory
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		if err := t.Scan(r, id); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

func (t *Trajectory) Predict(p, s time.Duration, saa Shape) ([]*Point, error) {
	if p < s {
		return nil, fmt.Errorf("period shorter than step (%s < %s)", p, s)
	}
	var rs []*Point
	sort.Slice(t.elements, func(i, j int) bool { return t.elements[i].When.Before(t.elements[j].When) })
	for i := 0; i < len(t.elements); i++ {
		curr := t.elements[i]
		period := p
		if j := i + 1; j < len(t.elements) {
			diff := t.elements[j].When.Sub(curr.When)
			period = diff
			p -= diff
		}
		log.Printf("trajectory prediction from %s to %s", curr.When, curr.When.Add(period))
		// TODO: first time should be the last time of previous element + by the step (s) value
		ps, err := curr.Predict(period, s, saa)
		if err != nil {
			return nil, err
		}
		rs = append(rs, ps...)
	}
	return rs, nil
}

func (t *Trajectory) Scan(r io.Reader,sid int) error {
	s := bufio.NewScanner(r)
	for s.Scan() {
		rs := make([]string, tleRows)
		for i := 0; i < len(rs); i++ {
			rs[i] = s.Text()
			if len(rs[i]) == emptyLen {
				// skip "empty" line added on celestrack
				if !s.Scan() {
					return MissingRowError(i)
				}
				rs[i] = s.Text()
			}
			if z := len(rs[i]); z != tleLen {
				return InvalidLenError(z)
			}
			if i == 0 && !s.Scan() {
				return MissingRowError(i)
			}
		}
		e, err := NewElement(rs[0], rs[1])
		if err != nil && e == nil {
			continue
		}
		if err != nil && e.Sid == sid {
			return err
		}
		if e.Sid == sid {
			t.elements = append(t.elements, e)
		}
	}
	return s.Err()
}

// func (t *Trajectory) Scan(r io.Reader, sid int) error {
// 	s := bufio.NewScanner(r)
// 	for {
// 		if !s.Scan() {
// 			break
// 		}
// 		x := s.Text()
// 		if strings.HasPrefix(x, "#") {
// 			continue
// 		}
// 		if len(x) == emptyLen {
// 			// TBD - ignore the empty line (name of satellite)
// 		}
// 		rs := make([]string, tleRows)
// 		for i := range rs {
// 			if !s.Scan() {
// 				return MissingRowError(i + 1)
// 			}
// 			rs[i] = s.Text()
// 			if n := len(rs[i]); n != tleLen {
// 				return InvalidLenError(n)
// 			}
// 		}
// 		e, err := NewElement(rs[0], rs[1])
// 		if err != nil && e == nil {
// 			continue
// 		}
// 		if err != nil && e.Sid == sid {
// 			return err
// 		}
// 		if e.Sid == sid {
// 			t.elements = append(t.elements, e)
// 		}
// 	}
// 	return s.Err()
// }
