package main

import (
	"encoding/csv"
	"io"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	Flattening   = 0.003352813178
	Excentricity = 0.006694385
	Radius       = 6378.136
)

func ReadPoints(files []string) (<-chan Point, error) {
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

			queue <- FromRow(row)
		}
	}()
	return queue, nil
}

type Point struct {
	When    time.Time
	Lat     float64
	Lng     float64
	Alt     float64
	Eclipse bool
	Saa     bool
}

func FromRow(row []string) Point {
	var pt Point

	pt.When, _ = time.Parse("2006-01-02T15:04:05.000000", row[0])
	pt.Alt, _ = strconv.ParseFloat(row[2], 64)
	pt.Lat, _ = strconv.ParseFloat(row[3], 64)
	pt.Lng, _ = strconv.ParseFloat(row[4], 64)
	pt.Eclipse, _ = strconv.ParseBool(row[5])
	pt.Saa, _ = strconv.ParseBool(row[6])

	return pt
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
