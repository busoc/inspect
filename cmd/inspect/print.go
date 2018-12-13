package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/celest"
)

type meta struct {
	TLE      int
	Points   int

	Crossing     int
	CrossingTime time.Duration

	Eclipse     int
	EclipseTime time.Duration
}

type printer struct {
	Format string `toml:"format"` // csv or pipe
	Syst   string `toml:"frames"` // geodetic, geocentric, teme
	DMS    bool   `toml:"toDMS"`  // convert to deg°min'sec'' NESW
	Round  bool   `toml:"to360"`  //360
}

func (pt printer) Print(w io.Writer, ps <-chan *celest.Result) (*meta, error) {
	switch strings.ToLower(pt.Format) {
	case "csv":
		return pt.printCSV(w, ps)
	case "", "pipe":
		return pt.printPipe(w, ps)
	default:
		return nil, fmt.Errorf("unsupported format %s", pt.Format)
	}
}

func (pt printer) rawFormat() bool {
	syst := strings.ToLower(pt.Syst)
	return syst == "teme" || syst == "eci" || syst == "ecef"
}

func (pt printer) transform(p *celest.Point) *celest.Point {
	return transform(p, pt.Syst)
}

func (pt printer) printCSV(w io.Writer, ps <-chan *celest.Result) (*meta, error) {
	fmt.Fprintf(w, "#%s-%s (build: %s)", Program, Version, BuildTime)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "#" + strings.Join(os.Args, " "))
	fmt.Fprintln(w, "#time, mjd, altitude, latitude, longitude, eclipse, saa, epoch")

	ws := csv.NewWriter(w)
	var m meta
	for r := range ps {
		m.TLE++
		m.Points += len(r.Points)
		fmt.Fprintf(w, "#%s", r.TLE[0])
		fmt.Fprintln(w)
		fmt.Fprintf(w, "#%s", r.TLE[1])
		fmt.Fprintln(w)

		if err := pt.printRow(ws, r, &m); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

func (pt printer) printRow(ws *csv.Writer, r *celest.Result, m *meta) error {
	var saa, eclipse *celest.Point
	for _, p := range r.Points {
		p = pt.transform(p)
		if p.Saa && saa == nil {
			saa = p
		}
		if !p.Saa && saa != nil {
			m.Crossing++
			saa = nil
		}
		if p.Total && eclipse == nil {
			eclipse = p
		}
		if !p.Total && eclipse != nil {
			m.Eclipse++
			eclipse = nil
		}
		if !pt.rawFormat() && pt.Round {
			p.Lon = math.Mod(p.Lon+360, 360)
		}
		rs := []string{
			p.When.Format("2006-01-02T15:04:05.000000"),
			strconv.FormatFloat(p.MJD(), 'f', -1, 64),
			strconv.FormatFloat(p.Alt, 'f', -1, 64),
			strconv.FormatFloat(p.Lat, 'f', -1, 64),
			strconv.FormatFloat(p.Lon, 'f', -1, 64),
			formatBool(p.Total),
			formatBool(p.Saa),
			strconv.FormatFloat(r.Epoch, 'f', -1, 64),
		}
		if err := ws.Write(rs); err != nil {
			return err
		}
	}
	ws.Flush()
	return ws.Error()
}

func formatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func (pt printer) printPipe(w io.Writer, ps <-chan *celest.Result) (*meta, error) {
	var row string
	if !pt.rawFormat() && pt.DMS {
		row = "%s | %.6f | %18.5f | %s | %s | %s | %s | %.6f"
	} else {
		row = "%s | %.6f | %18.5f | %18.5f | %18.5f | %s | %s | %.6f"
	}
	var m meta
	var saa, eclipse *celest.Point
	for r := range ps {
		m.TLE++
		m.Points += len(r.Points)
		for _, p := range r.Points {
			p = pt.transform(p)
			if p.Saa && saa == nil {
				saa = p
			}
			if !p.Saa && saa != nil {
				m.Crossing++
				saa = nil
			}
			if p.Total && eclipse == nil {
				eclipse = p
			}
			if !p.Total && eclipse != nil {
				m.Eclipse++
				eclipse = nil
			}
			if !pt.rawFormat() && pt.Round {
				p.Lon = math.Mod(p.Lon+360, 360)
			}
			var lat, lon interface{}
			if !pt.rawFormat() && pt.DMS {
				lat, lon = toDMS(p.Lat, "SN"), toDMS(p.Lon, "EW")
			} else {
				lat, lon = p.Lat, p.Lon
			}
			fmt.Fprintf(w, row, p.When.Format("2006-01-02 15:04:05.000000"), p.MJD(), p.Alt, lat, lon, formatBool(p.Total), formatBool(p.Saa), r.Epoch)
			fmt.Fprintln(w)
		}
	}
	return &m, nil
}

func toDMS(v float64, dir string) string {
	var deg, min, sec, rest float64
	deg, rest = math.Modf(v)
	min, sec = math.Modf(rest * 60)

	switch {
	case dir == "SN" && deg < 0:
		dir = "S"
	case dir == "SN" && deg >= 0:
		dir = "N"
	case dir == "EW" && deg < 0:
		dir = "W"
	case dir == "EW" && deg >= 0:
		dir = "E"
	}

	return fmt.Sprintf("%3d° %02d' %7.4f'' %s", int(math.Abs(deg)), int(math.Abs(min)), math.Abs(sec*60), dir)
}
