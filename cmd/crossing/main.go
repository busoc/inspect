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

	"github.com/midbel/linewriter"
	"github.com/midbel/toml"
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
	return fmt.Sprintf("crossing area: [%.3fS,%.3fN]x[%.3fW,%.3fE]", s.South, s.North, s.West, s.East)
}

type Eclipse bool

func (e Eclipse) Contains(pt Point) bool {
	ok := bool(e)
	return !ok || pt.Eclipse
}

func (e Eclipse) String() string {
	ok := bool(e)
	if ok {
		return "crossing night only passes"
	}
	return "crossing day and night passes"
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

const (
	Flattening   = 0.003352813178
	Excentricity = 0.006694385
	Radius       = 6378.136
)

type Point struct {
	When    time.Time
	Lat     float64
	Lng     float64
	Alt     float64
	Eclipse bool
	Saa     bool
}

func (p Point) Distance(t Point) float64 {
	x0, y0, z0 := p.Coordinates()
	x1, y1, z1 := t.Coordinates()

	diff := math.Pow(x1-x0, 2) + math.Pow(y1-y0, 2) + math.Pow(z1-z0, 2)
	return math.Sqrt(diff)
}

func (p Point) Coordinates() (float64, float64, float64) {
	var (
		rad = math.Pi / 180.0
		lat = p.Lat * rad
		lng = p.Lng * rad

		s = math.Pow(math.Sin(lat), 2)
		n = Radius * math.Pow((1-Flattening*(2-Flattening)*s), -0.5)

		x = (n + p.Alt) * math.Cos(lat) * math.Cos(lng)
		y = (n + p.Alt) * math.Cos(lat) * math.Sin(lng)
		z = (n*(1-Excentricity) + p.Alt) * math.Sin(lat)
	)
	return x, y, z
}

func main() {
	var (
		comma  = flag.Bool("csv", false, "csv")
		list   = flag.Bool("list", false, "list")
		lat    = flag.Float64("lat", 0, "latitude")
		lng    = flag.Float64("lng", 0, "longitude")
		mgn    = flag.Float64("margin", 10, "margin")
		night  = flag.Bool("night", false, "night")
		starts = flag.String("starts", "", "start time")
		ends   = flag.String("ends", "", "end time")
		config = flag.Bool("config", false, "use config file")
	)
	flag.Parse()

	if *config {

	} else {

	}

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
	ec := Eclipse(*night)

	fmt.Fprintln(os.Stderr, pd.String())
	fmt.Fprintln(os.Stderr, sq.String())
	fmt.Fprintln(os.Stderr, ec.String())

	var fn func(*linewriter.Writer, <-chan Point, Contain) error
	if *list {
		fn = asList
	} else {
		fn = asAggr
	}
	queue, err := readPoints(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	ws := Line(*comma)
	if err := fn(ws, queue, Contains(sq, pd, ec)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

type Area struct {
	Lat    float64 `toml:"latitude"`
	Lng    float64 `toml:"longitude"`
	Margin float64

	Night  bool

	Starts time.Time `toml:"dtstart"`
	Ends   time.Time `toml:"dtend"`
}

func (a Area) Contains() (Contain, error) {
	sq, err := NewSquare(a.Lat, a.Lng, a.Margin)
	if err != nil {
		return nil, err
	}
	pd := Period{
		Starts: a.Starts,
		Ends:   a.Ends,
	}
	return Contains(sq, pd, Eclipse(a.Night)), nil
}

type Setting struct {
	File  string
	List  bool
	Comma bool

	Areas []Area `toml:"area"`
}

func (s Setting) Contains() (Contain, error) {
	cs := make([]Contain, 0, len(s.Areas))
	for _, a := range s.Areas {
		c, err := a.Contains()
		if err != nil {
			return nil, err
		}
		cs = append(cs, c)
	}
	return Contains(cs...), nil
}

func configure() (Setting, error) {
	var s Setting
	return s, toml.DecodeFile(flag.Arg(0), &s)
}

func asAggr(ws *linewriter.Writer, queue <-chan Point, filter Contain) error {
	var (
		first Point
		last  Point
	)
	for pt := range queue {
		if filter.Contains(pt) {
			first, last = pt, pt
			for pt := range queue {
				if !filter.Contains(pt) {
					break
				}
				last = pt
			}

			var (
				delta = last.When.Sub(first.When)
				dist  = last.Distance(first)
			)

			for _, p := range []Point{first, last} {
				ws.AppendTime(p.When, "2006-01-02T15:04:05.00", linewriter.AlignLeft)
				ws.AppendFloat(p.Lat, 8, 3, linewriter.AlignRight|linewriter.Float)
				ws.AppendFloat(p.Lng, 8, 3, linewriter.AlignRight|linewriter.Float)
			}
			ws.AppendDuration(delta, 8, linewriter.AlignRight|linewriter.Second)
			ws.AppendFloat(dist, 8, 1, linewriter.AlignRight|linewriter.Float)

			if _, err := io.Copy(os.Stdout, ws); err != nil && err != io.EOF {
				return err
			}
		}
	}
	return nil
}

func asList(ws *linewriter.Writer, queue <-chan Point, filter Contain) error {
	for pt := range queue {
		if filter.Contains(pt) {
			ws.AppendTime(pt.When, "2006-01-02 15:05:04.00", linewriter.AlignLeft)
			ws.AppendFloat(pt.Alt, 8, 3, linewriter.AlignRight|linewriter.Float)
			ws.AppendFloat(pt.Lat, 8, 3, linewriter.AlignRight|linewriter.Float)
			ws.AppendFloat(pt.Lng, 8, 3, linewriter.AlignRight|linewriter.Float)

			if pt.Eclipse {
				ws.AppendString("night", 5, linewriter.AlignRight)
			} else {
				ws.AppendString("day", 5, linewriter.AlignRight)
			}
			if pt.Saa {
				ws.AppendString("saa", 3, linewriter.AlignRight)
			} else {
				ws.AppendString("-", 3, linewriter.AlignRight)
			}

			if _, err := io.Copy(os.Stdout, ws); err != nil && err != io.EOF {
				return err
			}
		}
	}
	return nil
}

func Line(comma bool) *linewriter.Writer {
	var opts []linewriter.Option
	if comma {
		opts = append(opts, linewriter.AsCSV(true))
	} else {
		opts = []linewriter.Option{
			linewriter.WithPadding([]byte(" ")),
			linewriter.WithSeparator([]byte("|")),
		}
	}
	return linewriter.NewWriter(8192, opts...)
}

func readPoints(files []string) (<-chan Point, error) {
	var (
		r  io.Reader
		rs []io.Reader
	)
	if len(files) == 0 {
		r = os.Stdin
	} else {
		rs = make([]io.Reader, 0, len(files))
		for _, a := range files {
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
			pt.Saa, _ = strconv.ParseBool(row[6])

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
