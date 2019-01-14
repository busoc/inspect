package celest

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"math"
	"sort"
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

func Open(files []string, id int, bstar float64) (*Trajectory, error) {
	var t Trajectory
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			return nil, err
		}
		defer r.Close()
		if err := t.Scan(r, id, bstar); err != nil {
			return nil, err
		}
	}
	return &t, nil
}

type Info struct {
	Sid int

	When   time.Time
	Starts time.Time
	Ends   time.Time
}

func (t *Trajectory) Infos(period, interval time.Duration) []*Info {
	var is []*Info
	sort.Slice(t.elements, func(i, j int) bool { return t.elements[i].When.Before(t.elements[j].When) })

	period += interval
	for x, e := range t.elements {
		if period <= 0 {
			break
		}
		i := Info{Sid: e.Sid, When: e.When}
		i.Starts = e.When.Add(interval).Truncate(interval)
		i.Ends = i.Starts.Add(period)
		if x < len(t.elements)-1 {
			n := t.elements[x+1]
			i.Ends = n.When.Add(interval).Truncate(interval).Add(interval)
		}
		is = append(is, &i)
		period -= i.Ends.Sub(i.Starts)
	}
	return is
}

func (t *Trajectory) Predict(p, s time.Duration, saa Shape, delay bool) (<-chan *Result, error) {
	if p < s {
		return nil, fmt.Errorf("period shorter than step (%s < %s)", p, s)
	}
	sort.Slice(t.elements, func(i, j int) bool { return t.elements[i].When.Before(t.elements[j].When) })
	q := make(chan *Result)
	go func() {
		defer close(q)
		for i := 0; i < len(t.elements); i++ {
			if p < 0 {
				return
			}
			curr := t.elements[i]
			period := p
			if len(t.elements) > 1 || delay {
				curr.Base = curr.When.Add(s).Truncate(s)
				if j := i+1; j < len(t.elements) {
					next := t.elements[j]
					period = next.When.Add(s).Truncate(s).Sub(curr.Base)
					p -= period
				}
			}
			r, err := curr.Predict(period, s, saa)
			r.When = curr.When
			if err != nil {
				log.Println(err)
				return
			}
			q <- r
		}
	}()
	return q, nil
}

func (t *Trajectory) Scan(r io.Reader, sid int, bstar float64) error {
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
		if math.Abs(e.BStar) > math.Abs(bstar) {
			return fmt.Errorf("bstar drag coefficient exceed limit: %.6f", e.BStar)
		}
		if e.Sid == sid {
			t.elements = append(t.elements, e)
		}
	}
	return s.Err()
}
