package celest

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"
)

const (
	emptyLen = 24
	tleLen   = 69
	tleRows  = 2
)

var (
	ErrShortPeriod = errors.New("propagation period shorter than step")
	ErrBaseTime    = errors.New("no propagation beyond base time")
)

type ParseError struct {
	row   int
	cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("fail to scan row #%d: %v", e.row, e.cause)
}

type PropagationError int

func (e PropagationError) Error() string {
	var msg string
	switch int(e) {
	case 1:
		msg = "mean elements, ecc >= 1.0 or ecc < -0.001 or a < 0.95"
	case 2:
		msg = "mean motion less than 0.0"
	case 3:
		msg = "pert elements, ecc < 0.0  or  ecc > 1.0"
	case 4:
		msg = "semi-latus rectum < 0.0"
	case 5:
		msg = "epoch elements are sub-orbital"
	case 6:
		msg = "satellite has decayed"
	default:
		msg = "propagation error"
	}
	return msg
}

type DragError float64

func (e DragError) Error() string {
	return fmt.Sprintf("bstar drag coefficient exceed limit: %.6f", float64(e))
}

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
	Base     time.Time
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
		return nil, ErrShortPeriod
	}
	sort.Slice(t.elements, func(i, j int) bool { return t.elements[i].When.Before(t.elements[j].When) })
	if !t.Base.IsZero() {
		var invalid bool
		for i := range t.elements {
			if t.elements[i].When.After(t.Base) {
				invalid = true
				break
			}
		}
		if invalid {
			return nil, ErrBaseTime
		}
	}
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
				if j := i + 1; j < len(t.elements) {
					next := t.elements[j]
					period = next.When.Add(s).Truncate(s).Sub(curr.Base)
					p -= period
				}
			}
			if !t.Base.IsZero() {
				s, e := curr.Range(period)
				if e.Before(t.Base) {
					continue
				}
				if s.Equal(t.Base) || s.Before(t.Base) {
					curr.Base = t.Base
				}
			}
			r, _ := curr.Predict(period, s, saa)
			r.When = curr.When
			q <- r
			if r.Err != nil {
				return
			}
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
			return DragError(e.BStar)
		}
		if e.Sid == sid {
			t.elements = append(t.elements, e)
		}
	}
	return s.Err()
}
