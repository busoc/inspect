package main

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/busoc/celest"
)

type encoder interface {
	Encode(interface{}) error
}

type printer struct {
	Format string `toml:"format"` // csv or pipe
	Syst   string `toml:"frames"` // geodetic, geocentric, teme
	DMS    bool   `toml:"toDMS"`  // convert to deg°min'sec'' NESW
	Round  bool   `toml:"to360"`  //360
}

func (pt printer) Print(w io.Writer, ps <-chan *celest.Result) error {
	switch strings.ToLower(pt.Format) {
	case "csv":
		return pt.printCSV(w, ps)
	case "", "pipe":
		return pt.printPipe(w, ps)
	case "xml":
		return pt.printEncoder(xml.NewEncoder(w), ps)
	case "json":
		return pt.printEncoder(json.NewEncoder(w), ps)
	default:
		return fmt.Errorf("unsupported format %s", pt.Format)
	}
}

func (pt printer) rawFormat() bool {
	syst := strings.ToLower(pt.Syst)
	return syst == "teme" || syst == "eci"
}

func (pt printer) transform(p *celest.Point) *celest.Point {
	switch strings.ToLower(pt.Syst) {
	default:
		g := p.Geocentric()
		return &g
	case "geodetic", "geodesic":
		g := p.Geodetic()
		return &g
	case "teme", "eci":
		return p
	}
}

func (pt printer) printEncoder(e encoder, ps <-chan *celest.Result) error {
	// for r := range ps {
	// 	if err := e.Encode(r); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}

func (pt printer) printCSV(w io.Writer, ps <-chan *celest.Result) error {
	ws := csv.NewWriter(w)
	for r := range ps {
		io.WriteString(w, fmt.Sprintf("#%s\n", r.TLE[0]))
		io.WriteString(w, fmt.Sprintf("#%s\n", r.TLE[1]))
		if err := pt.printRow(ws, r); err != nil {
			return err
		}
	}
	return nil
}

func (pt printer) printRow(ws *csv.Writer, r *celest.Result) error {
	for _, p := range r.Points {
		p = pt.transform(p)
		jd := celest.MJD50(p.When)
		var saa, eclipse int
		if p.Saa {
			saa++
		}
		if p.Total {
			eclipse++
		}
		if !pt.rawFormat() && pt.Round {
			p.Lon = math.Mod(p.Lon+360, 360)
		}
		rs := []string{
			p.When.Format("2006-01-02T15:04:05.000000"),
			strconv.FormatFloat(jd, 'f', -1, 64),
			strconv.FormatFloat(p.Alt, 'f', -1, 64),
			strconv.FormatFloat(p.Lat, 'f', -1, 64),
			strconv.FormatFloat(p.Lon, 'f', -1, 64),
			strconv.Itoa(eclipse),
			strconv.Itoa(saa),
			"-",
		}
		if err := ws.Write(rs); err != nil {
			return err
		}
	}
	ws.Flush()
	return ws.Error()
}

func (pt printer) printPipe(w io.Writer, ps <-chan *celest.Result) error {
	var row string
	if !pt.rawFormat() && pt.DMS {
		row = "%s | %.6f | %18.5f | %s | %s | %d | %d"
	} else {
		row = "%s | %.6f | %18.5f | %18.5f | %18.5f | %d | %d"
	}
	for r := range ps {
		for _, p := range r.Points {
			p = pt.transform(p)
			var saa, eclipse int
			if p.Saa {
				saa++
			}
			if p.Total {
				eclipse++
			}
			jd := celest.MJD50(p.When)
			if !pt.rawFormat() && pt.Round {
				p.Lon = math.Mod(p.Lon+360, 360)
			}
			var lat, lon interface{}
			if !pt.rawFormat() && pt.DMS {
				lat, lon = toDMS(p.Lat, "SN"), toDMS(p.Lon, "EW")
			} else {
				lat, lon = p.Lat, p.Lon
			}
			fmt.Fprintf(w, row, p.When.Format("2006-01-02 15:04:05.000000"), jd, p.Alt, lat, lon, eclipse, saa)
			fmt.Fprintln(w)
		}
	}
	return nil
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
