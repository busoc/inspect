package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
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

type Point struct {
	When    time.Time
	Lat     float64
	Lng     float64
	Alt     float64
	Eclipse bool

	Data []string
}

const (
	Flattening   = 0.003352813178
	Excentricity = 0.006694385
	Radius       = 6378.136
)

func (p Point) Distance(t Point) float64 {
	x0, y0, z0 := p.Coordinates()
	x1, y1, z1 := t.Coordinates()

	diff := math.Pow(x1-x0, 2) + math.Pow(y1-y0, 2) + math.Pow(z1-z0, 2)
	return math.Sqrt(diff)
}

func (p Point) Coordinates() (float64, float64, float64) {
	var (
		rad = math.Pi / 180.0

		s = math.Pow(math.Sin(p.Lat*rad), 2)
		n = math.Pow(Radius*(1-Flattening*(2-Flattening)*s), -0.5)

		x = (n + p.Alt) * math.Cos(p.Lat) * math.Cos(p.Lng)
		y = (n + p.Alt) * math.Cos(p.Lat) * math.Sin(p.Lng)
		z = (n*(1-Excentricity) + p.Alt) * math.Sin(p.Lat)
	)
	return x, y, z
}

func (p Point) IsZero() bool {
	return len(p.Data) == 0 && p.When.IsZero()
}

func main() {
	var (
		list   = flag.Bool("list", false, "list")
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

	fmt.Fprintln(os.Stderr, pd.String())
	fmt.Fprintln(os.Stderr, sq.String())

	var fn func(<-chan Point, Contain) error
	if *list {
		fn = asList
	} else {
		fn = asAggr
	}
	queue, err := readPoints()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := fn(queue, Contains(sq, pd, Eclipse(*night))); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func asAggr(queue <-chan Point, filter Contain) error {
	var (
		ws    = csv.NewWriter(os.Stdout)
		first Point
		last  Point
	)
	for pt := range queue {
		if filter.Contains(pt) {
			for pt := range queue {
				if !filter.Contains(pt) {
					last = pt
					break
				}
			}
			data := []string{
				first.Data[0],
				first.Data[3],
				first.Data[4],
				last.Data[0],
				last.Data[3],
				last.Data[4],
				last.When.Sub(first.When).String(),
				strconv.FormatFloat(last.Distance(first), 'f', -1, 64),
			}
			ws.Write(data)
		}
		first = pt
	}
	ws.Flush()
	return ws.Error()
}

func asList(queue <-chan Point, filter Contain) error {
	ws := csv.NewWriter(os.Stdout)
	for pt := range queue {
		if filter.Contains(pt) {
			ws.Write(pt.Data)
		}
	}
	ws.Flush()
	return ws.Error()
}

func readPoints() (<-chan Point, error) {
	var (
		r  io.Reader
		rs []io.Reader
	)
	if flag.NArg() == 0 {
		r = os.Stdin
	} else {
		rs = make([]io.Reader, 0, flag.NArg())
		for _, a := range flag.Args() {
			f, err := os.Open(a)
			if err != nil {
				return nil, err
			}
			rs = append(rs, f)
		}
		r = io.MultiReader(rs...)
	}
	queue := make(chan Point)
	go func() {
		defer func() {
			close(queue)
			for _, r := range rs {
				if c, ok := r.(io.Closer); ok {
					c.Close()
				}
			}
		}()

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
      pt.Alt, _ = strconv.ParseFloat(row[2], 64)
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
