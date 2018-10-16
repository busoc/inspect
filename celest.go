package main

import (
	"crypto/md5"
	"encoding/csv"
	"flag"
	"io"
	"log"
	"os"
	"net/http"
	"strings"
	"time"
	"strconv"
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
	Name      = "inspect"
	Version   = "0.0.1"
	BuildTime = "2018-10-16 11:10:00"
)

func main() {
	format := flag.String("f", "", "output format")
	sid := flag.Int("s", DefaultSid, "satellite number")
	period := flag.Duration("d", time.Hour*72, "time range")
	interval := flag.Duration("i", time.Minute, "time interval")
	flag.Parse()

	var write func(io.Writer, []*Point) error
	switch strings.ToLower(*format) {
	case "csv":
		write = writeCSV
	case "", "pipe":
		write = writePipe
	default:
		log.Fatalln("unsupported output format: %s", *format)
	}

	var t Trajectory
	digest := md5.New()
	if resp, err := http.Get(flag.Arg(0)); err == nil {
		if resp.StatusCode != http.StatusOK {
			log.Fatalf("fail to fetch data from %s (%s)", flag.Arg(0), resp.Status)
		}
		if err := t.Scan(io.TeeReader(resp.Body, digest), *sid); err != nil {
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
			log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", flag.Arg(0), digest.Sum(nil), s.ModTime().Format(time.RFC1123))
			r.Close()
			digest.Reset()
		}
	}

	ps, err := t.Predict(*period, *interval)
	// log.Printf("trajectory: %d positions", len(ps))
	if err != nil {
		log.Fatalln(err)
	}
	if err := write(os.Stdout, ps); err != nil {
		log.Fatalln(err)
	}
}

func writeCSV(w io.Writer, ps []*Point) error {
	ws := csv.NewWriter(w)
	for _, p := range ps {
		jd, _, _ := mjdTime(p.When)
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
			strconv.FormatFloat(p.Alt/1000.0, 'f', -1, 64),
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

func writePipe(w io.Writer, ps []*Point) error {
	const row = "%s | %.2f | %18.5f | %18.5f | %18.5f | %d | %d"
	logger := log.New(w, "", 0)
	for _, p := range ps {
		var saa, eclipse int
		if p.Saa {
			saa++
		}
		if p.Total {
			eclipse++
		}
		jd, _, _ := mjdTime(p.When)
		logger.Printf(row, p.When.Format("2006-01-02 15:04:05"), jd, p.Alt/1000.0, p.Lat, p.Lon, eclipse, saa)
	}
	return nil
}
