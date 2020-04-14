package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
  "strconv"
)

type Square struct {
	East  float64
	West  float64
	North float64
	South float64
}

type Point struct {
  Lat float64
  Lng float64
  Eclipse bool

  Data []string
}

func NewSquare(lat, lng, margin float64) Square {
	return Square{
		East:  lng - margin,
		West:  lng + margin,
		North: lat + margin,
		South: lat - margin,
	}
}

func (s Square) Contains(pt Point) bool {
	ns := pt.Lat >= s.South && pt.Lat <= s.North
  ew := pt.Lng >= s.East && pt.Lng <= s.West

  return ns && ew
}

func main() {
	var (
		lat = flag.Float64("lat", 0, "latitude")
		lng = flag.Float64("lng", 0, "longitude")
		mgn = flag.Float64("margin", 10, "margin")
    night = flag.Bool("night", false, "night")
	)
	flag.Parse()

	sq := NewSquare(*lat, *lng, *mgn)
  ws := csv.NewWriter(os.Stdout)

  queue, err := ReadPoints()
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(2)
  }
  for pt := range queue {
    if sq.Contains(pt) {
      if *night && !pt.Eclipse {
        continue
      }
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
      pt.Lat, _ = strconv.ParseFloat(row[3], 64)
      pt.Lng, _ = strconv.ParseFloat(row[4], 64)
      pt.Eclipse, _ =  strconv.ParseBool(row[5])

      pt.Data = row

      queue <- pt
  	}
  }()
  return queue, nil
}
