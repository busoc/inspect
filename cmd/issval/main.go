package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/busoc/celest"
)

const DiffKM = 100

const (
	TimeFormat          = "2006-01-02 15:04:05"
	PredictTimeFormat   = "2006-01-02T15:04:05.000000"
	PredictTimeIndex    = 0
	PredictIndexZ       = 2
	PredictIndexX       = 3
	PredictIndexY       = 4
	PredictEclipseIndex = 5
	PredictSaaIndex     = 6
	PredictColumns      = 8
	PredictComma        = ','
	PredictComment      = '#'
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	keep := flag.Bool("k", false, "keep xyz coordinates")
	other := flag.String("t", "", "compare with propagated trajectory")
	flag.Parse()

	var err error
	switch *other {
	default:
		r, e := os.Open(*other)
		if e != nil {
			err = e
			break
		}
		defer r.Close()
		err = comparePoints(r, flag.Args(), *keep)
	case "-":
		err = comparePoints(os.Stdin, flag.Args(), *keep)
	case "":
		listPoints(flag.Args(), *keep)
	}
	if err != nil {
		log.Fatalln(err)
	}
}

type delta struct {
	X    float64
	Y    float64
	Z    float64
	Raw  bool
	Dist float64
}

func comparePoints(r io.Reader, ps []string, keep bool) error {
	rs := csv.NewReader(r)
	rs.Comment = PredictComment
	rs.Comma = PredictComma
	rs.FieldsPerRecord = PredictColumns

	for p := range fetchPoints(ps) {
		var c *Point
		for {
			n, err := pointFromReader(rs)
			if err != nil || n == nil {
				return err
			}
			if p.When.Truncate(time.Second).Equal(n.When) {
				c = n
				break
			}
		}
		var dist float64
		if keep {
			dist = c.Cartesian(*p)
		} else {
			x := p.Convert()
			dist = c.Haversin(*x)
		}
		log.Printf("%s | %s | %12.5fkm | %t | %t", c.When.Format(TimeFormat), p.When.Format(TimeFormat), dist, c.Eclipse == p.Eclipse, c.Saa == p.Saa)
	}
	return nil
}

func listPoints(ps []string, keep bool) {
	for p := range fetchPoints(flag.Args()) {
		var saa, eclipse int
		if p.Saa {
			saa++
		}
		if p.Eclipse {
			eclipse++
		}
		if !keep {
			p = p.Convert()
			p.Alt /= 1000
		}
		log.Printf("%s | %18.5f | %18.5f | %18.5f | %d | %d", p.When.Format(TimeFormat), p.Alt, p.Lat, p.Lon, eclipse, saa)
	}
}

const FeetTo = 3280.841

var (
	Leap  = 18 * time.Second
	UNIX  = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	GPS   = time.Date(1980, 1, 6, 0, 0, 0, 0, time.UTC)
	Delta = GPS.Sub(UNIX) - Leap
)

const earthRadius = 6378.136

type Point struct {
	When time.Time

	Lat float64
	Lon float64
	Alt float64

	Saa     bool
	Eclipse bool
}

func (p *Point) Convert() *Point {
	lat, lon, alt := celest.ConvertTEME(p.When, []float64{p.Lat, p.Lon, p.Alt})
	return &Point{
		When:    p.When,
		Lat:     lat,
		Lon:     lon,
		Alt:     alt,
		Saa:     p.Saa,
		Eclipse: p.Eclipse,
	}
}

func (p Point) Haversin(r Point) float64 {
	plat, plon := p.Lat*(math.Pi/180), p.Lon*(math.Pi/180)
	rlat, rlon := r.Lat*(math.Pi/180), r.Lon*(math.Pi/180)

	deltaLat := (rlat - plat) / 2
	deltaLon := (rlon - plon) / 2
	a := math.Pow(math.Sin(deltaLat), 2) + (math.Cos(plat) * math.Cos(rlon) * math.Pow(math.Sin(deltaLon), 2))
	return 2 * earthRadius * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func (p Point) Cartesian(r Point) float64 {
	x := math.Pow(p.Lat-r.Lat, 2)
	y := math.Pow(p.Lon-r.Lon, 2)
	z := math.Pow(p.Alt-r.Alt, 2)
	return math.Sqrt(x + y + z)
}

func fetchPoints(ps []string) <-chan *Point {
	q := make(chan *Point)
	go func() {
		defer close(q)
		for _, p := range ps {
			err := filepath.Walk(p, func(p string, i os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if i.IsDir() {
					return nil
				}
				r, err := os.Open(p)
				if err != nil {
					return err
				}
				defer r.Close()
				ps, err := readPoints(r)
				if err == nil {
					for _, p := range ps {
						q <- p
					}
				}
				return err
			})
			if err != nil {
				return
			}
		}
	}()
	return q
}

// BAD_GNC_Inert_(X|Y|Z)
var (
	UMIPosX = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x68} //0x0C0000008768
	UMIPosY = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x69} //0x0C0000008769
	UMIPosZ = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x6A} //0x0C000000876A
)

var (
	UMISaaStat = []byte{0x0C, 0x00, 0x00, 0x00, 0x8E, 0xEE} //0x0C0000008EEE
	UMISunStat = []byte{0x0C, 0x00, 0x00, 0x00, 0x8E, 0xEF} //0x0C0000008EEF
)

const (
	SaaInFlag = "IN"
	SunInFlag = "NIGHT"
)

func readPoints(r io.Reader) ([]*Point, error) {
	s := bufio.NewScanner(r)
	s.Split(scanPackets)

	var (
		ps   []*Point
		curr *Point
	)
	for s.Scan() {
		bs := s.Bytes()
		if bs[0] != 2 {
			continue
		}
		coarse := binary.BigEndian.Uint32(bs[14:])
		fine := uint8(bs[18])

		w := readTime5(coarse, fine).Truncate(time.Second)
		switch xs := bs[5:11]; {
		case bytes.Equal(xs, UMIPosX):
			tmp := &Point{When: w.Add(Delta)}
			if curr != nil {
				tmp.Saa = curr.Saa
				tmp.Eclipse = curr.Eclipse
			}
			curr = tmp
			ps = append(ps, curr)
			curr.Lat = readFloat(bs[21:])
		case bytes.Equal(xs, UMIPosY) && curr != nil:
			curr.Lon = readFloat(bs[21:])
		case bytes.Equal(xs, UMIPosZ) && curr != nil:
			curr.Alt = readFloat(bs[21:])
		case bytes.Equal(xs, UMISaaStat) && curr != nil:
			curr.Saa = string(bytes.Trim(bs[21:], "\x00")) == SaaInFlag
		case bytes.Equal(xs, UMISunStat) && curr != nil:
			curr.Eclipse = string(bytes.Trim(bs[21:], "\x00")) == SunInFlag
		default:
			continue
		}
	}
	return ps, s.Err()
}

func readFloat(bs []byte) float64 {
	v := binary.BigEndian.Uint64(bs)
	return math.Float64frombits(v) / FeetTo
}

func scanPackets(bs []byte, ateof bool) (int, []byte, error) {
	if len(bs) < 4 {
		return 0, nil, nil
	}
	s := int(binary.LittleEndian.Uint32(bs[:4]))
	if s+4 >= len(bs) {
		return 0, nil, nil
	}
	vs := make([]byte, s)
	return copy(vs, bs[4:4+s]) + 4, vs, nil
}

func readTime5(coarse uint32, fine uint8) time.Time {
	t := time.Unix(int64(coarse), 0).UTC()

	fs := float64(fine) / 256.0 * 1000.0
	ms := time.Duration(fs) * time.Millisecond
	return t.Add(ms).UTC()
}

func pointFromReader(rs *csv.Reader) (*Point, error) {
	vs, err := rs.Read()
	if vs == nil && err == io.EOF {
		return nil, nil
	}
	if err != nil && err != io.EOF {
		return nil, err
	}

	var pt Point
	if pt.When, err = time.Parse(PredictTimeFormat, vs[PredictTimeIndex]); err != nil {
		return nil, err
	}
	pt.When = pt.When.Truncate(time.Second)
	if pt.Alt, err = strconv.ParseFloat(vs[PredictIndexZ], 64); err != nil {
		return nil, err
	}
	if pt.Lat, err = strconv.ParseFloat(vs[PredictIndexX], 64); err != nil {
		return nil, err
	}
	if pt.Lon, err = strconv.ParseFloat(vs[PredictIndexY], 64); err != nil {
		return nil, err
	}
	return &pt, err
}