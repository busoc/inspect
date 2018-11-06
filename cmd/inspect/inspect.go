package main

import (
	"crypto/md5"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/busoc/celest"
)

const (
	SAALatMin = -60.0
	SAALatMax = -5.0
	SAALonMin = -80.0
	SAALonMax = 40.0
)

const (
	Y2000       = 2000
	Y1900       = 1900
	YPivot      = 57
	secPerMins  = 60.0
	secPerHours = secPerMins * secPerMins
	secPerDays  = secPerHours * 24
	minPerDays  = 1440.0
)

const DefaultSid = 25544

const (
	Program   = "inspect"
	Version   = "0.0.1"
	BuildTime = "2018-10-16 11:10:00"
)

type rect struct {
	North float64
	South float64
	West  float64
	East  float64
}

func (r *rect) Contains(p celest.Point) bool {
	return (p.Lat > r.South && p.Lat < r.North) && (p.Lon > r.West && p.Lon < r.East)
}

func (r *rect) Set(s string) error {
	_, err := fmt.Sscanf(s, "%f:%f:%f:%f", &r.North, &r.East, &r.South, &r.West)
	return err
}

func (r *rect) String() string {
	return fmt.Sprintf("rect(%.2fN:%.2fE:%.2fS:%.2fW)", r.North, r.East, r.South, r.West)
}

func init() {
	log.SetPrefix(fmt.Sprintf("[%s-%s] ", Program, Version))
}

func main() {
	saa := rect{
		North: SAALatMax,
		South: SAALatMin,
		East:  SAALonMax,
		West:  SAALonMin,
	}
	flag.Var(&saa, "r", "saa area")
	copydir := flag.String("t", os.TempDir(), "temp dir")
	format := flag.String("f", "pipe", "output format")
	sid := flag.Int("s", DefaultSid, "satellite number")
	period := flag.Duration("d", time.Hour*72, "time range")
	interval := flag.Duration("i", time.Minute, "time interval")
	file := flag.String("w", "", "write trajectory to file (stdout if not provided)")
	teme := flag.Bool("k", false, "keep TEME coordonates")
	round := flag.Bool("360", false, "round")
	flag.Parse()

	var write func(io.Writer, bool, bool, <-chan *celest.Result) error
	switch strings.ToLower(*format) {
	case "csv":
		write = writeCSV
	case "", "pipe":
		write = writePipe
	default:
		log.Fatalln("unsupported output format: %s", *format)
	}

	var t celest.Trajectory
	digest := md5.New()
	if resp, err := http.Get(flag.Arg(0)); err == nil {
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("fail to fetch data from %s (%s)", flag.Arg(0), resp.Status)
		}
		var w io.Writer = digest
		suffix := "-" + time.Now().Format("20060102_150405")
		if err := os.MkdirAll(*copydir, 0755); err != nil && !os.IsExist(err) {
			log.Fatalln(err)
		}
		if f, err := os.Create(path.Join(*copydir, path.Base(flag.Arg(0)+suffix))); err == nil {
			defer f.Close()
			w = io.MultiWriter(f, w)
		}
		if err := t.Scan(io.TeeReader(resp.Body, w), *sid); err != nil {
			log.Fatalln(err)
		}
		log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", flag.Arg(0), digest.Sum(nil), resp.Header.Get("last-modified"))
		resp.Body.Close()
	} else {
		for _, p := range flag.Args() {
			r, err := os.Open(p)
			if err != nil {
				log.Fatalln(err)
			}
			if err := t.Scan(io.TeeReader(r, digest), *sid); err != nil {
				log.Fatalln(err)
			}
			s, _ := r.Stat()
			log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", p, digest.Sum(nil), s.ModTime().Format(time.RFC1123))
			r.Close()
			digest.Reset()
		}
	}

	ps, err := t.Predict(*period, *interval, *teme, &saa)
	if err != nil {
		log.Fatalln(err)
	}
	var w io.Writer = os.Stdout
	switch f, err := os.Create(*file); {
	case err == nil:
		defer f.Close()
		w = f
	case err != nil && *file != "":
		log.Fatalln(err)
	}
	if err := write(w, *teme, *round, ps); err != nil {
		log.Fatalln(err)
	}
}

func writeCSV(w io.Writer, teme, _ bool, ps <-chan *celest.Result) error {
	div := 1.0
	if !teme {
		div = 1000
	}
	ws := csv.NewWriter(w)
	for r := range ps {
		io.WriteString(w, fmt.Sprintf("#%s\n", r.TLE[0]))
		io.WriteString(w, fmt.Sprintf("#%s\n", r.TLE[1]))
		for _, p := range r.Points {
			jd := celest.MJD70(p.When)
			var saa, eclipse int
			if p.Saa {
				saa++
			}
			if p.Total {
				eclipse++
			}
			rs := []string{
				p.When.Format("2006-01-02T15:04:05.000000"),
				strconv.FormatFloat(jd, 'f', -1, 64),
				strconv.FormatFloat(p.Alt/div, 'f', -1, 64),
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
	}
	ws.Flush()
	return ws.Error()
}

func writePipe(w io.Writer, teme, round bool, ps <-chan *celest.Result) error {
	div := 1.0
	if !teme {
		div = 1000
	}
	const row = "%s | %.6f | %18.5f | %18.5f | %18.5f | %d | %d"
	logger := log.New(w, "", 0)
	for r := range ps {
		for _, p := range r.Points {
			var saa, eclipse int
			if p.Saa {
				saa++
			}
			if p.Total {
				eclipse++
			}
			jd := celest.MJD50(p.When)
			if round && !teme {
				p.Lon += 360
			}
			logger.Printf(row, p.When.Format("2006-01-02 15:04:05.000000"), jd, p.Alt/div, p.Lat, p.Lon, eclipse, saa)
		}
	}
	return nil
}
