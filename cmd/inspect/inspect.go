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
	BuildTime = "2018-11-19 09:00:00"
)

const helpText = `Satellite trajectory prediction tool with Eclipse and SAA crossing.

Usage: inspect [-c] [-d] [-i] [-f] [-r] [-s] [-t] [-w] [-360] [-dms] <tle,...>

inspect allows to calculate from a set of (locale or remote) TLE (two line elements
set) a trajectory for a given satellite. To predict the path of a satellite, it
use the SGP4 library written by D. Vallado in C++.

The predicted trajectory given by inspect computes each point independantly of the
previous. The consequence is that the results of inspect could be "quickly" different
from other prediction tools that use a different method of predicting the trajectory
from the same TLE and satellite.

Coordinate systems/frames:

inspect can give the position of a satellite in three different way:

- geocentric: the latitude, longitude and altitude are given from the centre
of the earth. The values are given in degrees for the latitude and longitude,
and kilometer for the altitude. These values are derived from the geodetic values.

- geodetic: the latitude, longitude and altitude are given above an ellipsoidal
surface of the earth. The values are given in degrees for the latitude and
longitude, and kilometer for the altitude.

- teme/eci: the latitude, longitude and altitude are given from the centre of the
earth and are given in kilometer. The main difference is that in this reference
system, the values are computed in an inertial system that do not rotate with
the earth. These values are the one given by the SGP4 propagator used by inspect.

TLE/Input format:

inspect can only support the following TLE format (the first line being optional.
But if present should be 24 characters long)

ISS (ZARYA)
1 25544U 98067A   18304.35926896  .00001207  00000-0  25703-4 0  9995
2 25544  51.6420  60.1332 0004268 356.0118  61.1534 15.53880871139693

Output format:

the output of inspect consists of a tabulated "file". The columns in the result are:

- datetime (YYYY-mm-dd HH:MM:SS.ssssss)
- modified julian day
- altitude (kilometer)
- latitude (degree or DMS)
- longitude (degree or DMS)
- eclipse (1: night, 0: day)
- crossing (1: crossing, 0: no crossing)
- TLE epoch (not printed when output is pipe separated)

Options:

  -c   COORD   coordinate system used (geocentric, geodetic, teme/eci)
  -d   TIME    TIME over which calculate the predicted trajectory
  -f   FORMAT  print predicted trajectory in FORMAT (csv, pipe, json, xml)
  -i   TIME    TIME between two points on the predicted trajectory
  -r   AREA    check if the predicted trajectory crossed the given AREA
  -s   SID     satellite identifier
  -t   DIR     store a TLE fetched from a remote server in DIR
  -w   FILE    write predicted trajectory in FILE (default to stdout)
  -360         longitude are given in range of [0:360[ instead of ]-180:180[
  -dms         convert latitude and longitude to DD°MIN'SEC'' format

Examples:

# calcule the predicted trajectory on 24h for the default satellite from the latest
# TLE available on celestrack
$ inspect -d 24h -i 10s https://celestrak.com/NORAD/elements/stations.txt

# calculate the predicted trajectory on 24h for the default satellite from a locale
# TLE
$ inspect -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt

# calculate the predicted trajectory on 24h for the default satellite with 1 minute
# between two points of the path. The positions will be computed according to the
# geodetic system and printed as DD°MM'SS'. Moreover, it will check if the satellite
# cross a rectangle draw above a small town in Belgium instead of the SAA.
$ inspect -r 51.0:46.0:49.0:50 -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt
`

func init() {
	log.SetPrefix(fmt.Sprintf("[%s-%s] ", Program, Version))
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, helpText)
		os.Exit(2)
	}
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

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
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
