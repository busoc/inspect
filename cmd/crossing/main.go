package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type Contain interface {
	Contains(Point) bool
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

func (p Period) Contains(pt Point) bool {
	if !p.Starts.IsZero() && pt.When.Before(p.Starts) {
		return false
	}
	return p.Ends.IsZero() || pt.When.Before(p.Ends)
}

func (p Period) String() string {
	if p.Starts.IsZero() && p.Ends.IsZero() {
		return "crossing time range [,]"
	}
	return fmt.Sprintf("crossing time range [%s,%s]", p.Starts.Format(time.RFC3339), p.Ends.Format(time.RFC3339))
}

type Square struct {
	East  float64
	West  float64
	North float64
	South float64
}

type Point struct {
	When    time.Time
	Lat     float64
	Lng     float64
	Eclipse bool

	Data []string
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

func (s Square) Contains(pt Point) bool {
	ns := pt.Lat >= s.South && pt.Lat <= s.North
	ew := pt.Lng >= s.East && pt.Lng <= s.West

	return ns && ew
}

func (s Square) String() string {
	return fmt.Sprintf("crossing area: [%.3fS,%.3fN]x[%.3fE,%.3fW]", s.South, s.North, s.East, s.West)
}

type Eclipse bool

func (e Eclipse) Contains(pt Point) bool {
	ok := bool(e)
	return !ok || pt.Eclipse
}

func Contains(cs ...Contain) Contain {
	vs := make([]Contain, len(cs))
	copy(vs, cs)
	return Filter{vs}
}

type Filter struct {
	cs []Contain
}

func (f Filter) Contains(pt Point) bool {
	for _, c := range f.cs {
		ok := c.Contains(pt)
		if !ok {
			return ok
		}
	}
	return true
}

func main() {
	var (
		lat    = flag.Float64("lat", 0, "latitude")
		lng    = flag.Float64("lng", 0, "longitude")
		mgn    = flag.Float64("margin", 10, "margin")
		night  = flag.Bool("night", false, "night")
		starts = flag.String("starts", "", "start time")
		ends   = flag.String("ends", "", "end time")
	)
	flag.Parse()

	sq, err := NewSquare(*lat, *lng, *mgn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	pd, err := NewPeriod(*starts, *ends)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, sq.String())
	ws := csv.NewWriter(os.Stdout)

	filter := Contains(sq, pd, Eclipse(*night))

	queue, err := ReadPoints()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for pt := range queue {
		if filter.Contains(pt) {
			ws.Write(pt.Data)
		}
	}
	ws.Flush()
	if err := ws.Error(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func ReadPoints() (<-chan Point, error) {
	var r io.Reader
	if flag.NArg() == 0 {
		r = os.Stdin
	} else {
		rs := make([]io.Reader, 0, flag.NArg())
		for _, a := range flag.Args() {
			f, err := os.Open(a)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			rs = append(rs, f)
		}
		r = io.MultiReader(rs...)
	}
	queue := make(chan Point)
	go func() {
		defer close(queue)

		rs := csv.NewReader(r)
		rs.Comment = '#'
		rs.Comma = ','
		rs.FieldsPerRecord = 8
		for {
			row, err := rs.Read()
			if err != nil {
				break
			}
			var pt Point
			pt.When, _ = time.Parse("2006-01-02T15:04:05.000000", row[0])
			pt.Lat, _ = strconv.ParseFloat(row[3], 64)
			pt.Lng, _ = strconv.ParseFloat(row[4], 64)
			pt.Eclipse, _ = strconv.ParseBool(row[5])

			pt.Data = row

			queue <- pt
		}
	}()
	return queue, nil
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
