package main

import (
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/busoc/celest"
)

const (
	SAALatMin = -60.0
	SAALatMax = -5.0
	SAALonMin = -80.0
	SAALonMax = 40.0
)

const DefaultSid = 25544

const (
	Program   = "inspect"
	Version   = "0.0.1-dev"
	BuildTime = "2018-11-13 09:21:00"
)

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

	var p printer
	flag.Var(&saa, "r", "saa area")
	copydir := flag.String("t", os.TempDir(), "temp dir")
	sid := flag.Int("s", DefaultSid, "satellite number")
	period := flag.Duration("d", time.Hour*72, "time range")
	interval := flag.Duration("i", time.Minute, "time interval")
	file := flag.String("w", "", "write trajectory to file (stdout if not provided)")
	flag.StringVar(&p.Format, "f", "", "format")
	flag.StringVar(&p.Syst, "c", "", "system")
	flag.BoolVar(&p.Round, "360", false, "round")
	flag.BoolVar(&p.DMS, "dms", false, "dms")
	flag.Parse()

	t, err := fetchTLE(flag.Args(), *copydir, *sid)
	if err != nil {
		log.Fatalln(err)
	}
	rs, err := t.Predict(*period, *interval, &saa)
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
	if err := p.Print(w, rs); err != nil {
		log.Fatalln(err)
	}
}

func fetchTLE(ps []string, copydir string, sid int) (*celest.Trajectory, error) {
	var t celest.Trajectory
	digest := md5.New()
	if resp, err := http.Get(ps[0]); err == nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fail to fetch data from %s (%s)", ps[0], resp.Status)
		}
		var w io.Writer = digest
		suffix := "-" + time.Now().Format("20060102_150405")
		if err := os.MkdirAll(copydir, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}
		if f, err := os.Create(path.Join(copydir, path.Base(ps[0]+suffix))); err == nil {
			defer f.Close()
			w = io.MultiWriter(f, w)
		}
		if err := t.Scan(io.TeeReader(resp.Body, w), sid); err != nil {
			return nil, err
		}
		log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", ps[0], digest.Sum(nil), resp.Header.Get("last-modified"))
		resp.Body.Close()
	} else {
		for _, p := range ps {
			r, err := os.Open(p)
			if err != nil {
				return nil, err
			}
			if err := t.Scan(io.TeeReader(r, digest), sid); err != nil {
				return nil, err
			}
			s, _ := r.Stat()
			log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", p, digest.Sum(nil), s.ModTime().Format(time.RFC1123))
			r.Close()
			digest.Reset()
		}
	}
	return &t, nil
}
