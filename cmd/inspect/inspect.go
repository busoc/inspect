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
	"strings"
	"time"

	"github.com/busoc/celest"
	"github.com/midbel/toml"
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
	Version   = "1.0.0"
	BuildTime = "2018-11-28 13:55:00"
)

const helpText = `Satellite trajectory prediction tool with Eclipse and SAA crossing.

Usage: inspect [-c] [-d] [-i] [-f] [-r] [-s] [-t] [-w] [-360] [-dms] <tle,...>

inspect calculates the trajectory of a given satellite from a set of (local or
remote) TLE (two-line elements set). To predict the path of a satellite, it uses
the SGP4 library written by D. Vallado in C++.

The predicted trajectory given by inspect computes each point independantly from
the previous, unlike other propagation methods.

Coordinate systems/frames:

inspect can give the position of a satellite in three different way (mutually
exclusive):

- geocentric: the latitude, longitude and altitude are calculated from the centre
of the earth.

- geodetic: the latitude, longitude and altitude are calculated above an ellipsoidal
surface of the earth.

- teme/eci: the latitude, longitude and altitude are calculated from the centre of the
earth. The main difference is that, in this frame, the values are computed in an
inertial system that do not rotate with the earth. These values are the outcome
of the SGP4 propagator used by inspect and are used the computed the latitude,
longitude in the geodetic or geocentric frame.

TLE/Input format:

inspect can only support the following TLE format (the first line being optional.
But if present should be 24 characters long)

ISS (ZARYA)
1 25544U 98067A   18304.35926896  .00001207  00000-0  25703-4 0  9995
2 25544  51.6420  60.1332 0004268 356.0118  61.1534 15.53880871139693

Output format:


the output of inspect consists of a tabulated "file". The columns of the file are:

- datetime (YYYY-mm-dd HH:MM:SS.ssssss)
- modified julian day
- altitude (kilometer)
- latitude (degree or DMS)
- longitude (degree or DMS)
- eclipse (1: night, 0: day)
- crossing (1: crossing, 0: no crossing)
- TLE epoch (not printed when output is pipe separated)

Options:

  -c       COORD   coordinate system used (geocentric, geodetic, teme/eci)
  -d       TIME    TIME over which calculate the predicted trajectory
  -f       FORMAT  print predicted trajectory in FORMAT (csv, pipe, json, xml)
  -i       TIME    TIME between two points on the predicted trajectory
  -r       AREA    check if the predicted trajectory crossed the given AREA
  -s       SID     satellite identifier
  -t       DIR     store a TLE fetched from a remote server in DIR
  -w       FILE    write predicted trajectory in FILE (default to stdout)
  -360             longitude are given in range of [0:360[ instead of ]-180:180[
  -dms             convert latitude and longitude to DD°MIN'SEC'' format
  -config          load settings from a configuration file
	-version         print inspect version and exit
  -info            print info about the given TLE
  -help            print this message and exit

Examples:

# calculate the predicted trajectory over 24h for the default satellite from the
# latest TLE available on celestrak
$ inspect -d 24h -i 10s https://celestrak.com/NORAD/elements/stations.txt

# calculate the predicted trajectory over 24h for the default satellite from a
# locale TLE
$ inspect -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt

# calculate the predicted trajectory over 72h for the default satellite with 1 minute
# between two points of the path. The positions will be computed according to the
# geodetic system and printed as DD°MM'SS'. Moreover, it will check if the satellite
# cross a rectangle draw above a small town in Belgium.
$ inspect -r 51.0:46.0:49.0:50 -c geodetic -dms -d 72h -i 1m /tmp/tle-201481119.txt

# use a configuration file instead of command line options
$ inspect -config etc/inspect.toml

# use inspect as a REST service to generate trajectory from HTTP requests
$ inspect -listen 0.0.0.0:1911
`

type Duration struct {
	time.Duration
}

func (d *Duration) Set(s string) error {
	v, err := time.ParseDuration(s)
	if err == nil {
		d.Duration = v
	}
	return err
}

func (d *Duration) String() string {
	return d.Duration.String()
}

func init() {
	log.SetOutput(os.Stderr)
	log.SetPrefix(fmt.Sprintf("[%s-%s] ", Program, Version))
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, helpText)
		os.Exit(2)
	}
}

type Settings struct {
	Area     rect     `toml:"area"`
	File     string   `toml:"file"`
	Source   string   `toml:"tle"`
	Temp     string   `toml:"tmpdir"`
	Sid      int      `toml:"satellite"`
	Period   Duration `toml:"duration"`
	Interval Duration `toml:"interval"`

	Print printer `toml:"format"`
}

func (s *Settings) Update(f string) error {
	r, err := os.Open(f)
	if err != nil {
		return err
	}
	defer r.Close()

	return toml.NewDecoder(r).Decode(s)
}

func main() {
	s := Settings{
		Area:     SAA,
		Temp:     os.TempDir(),
		Sid:      DefaultSid,
		Period:   Duration{time.Hour * 72},
		Interval: Duration{time.Minute},
	}

	flag.StringVar(&s.Print.Format, "f", "", "format")
	flag.StringVar(&s.Print.Syst, "c", "", "system")
	flag.BoolVar(&s.Print.Round, "360", false, "round")
	flag.BoolVar(&s.Print.DMS, "dms", false, "dms")
	flag.Var(&s.Area, "r", "saa area")
	flag.StringVar(&s.Temp, "t", s.Temp, "temp dir")
	flag.IntVar(&s.Sid, "s", s.Sid, "satellite number")
	flag.Var(&s.Period, "d", "time range")
	flag.Var(&s.Interval, "i", "time interval")
	flag.StringVar(&s.File, "w", "", "write trajectory to file (stdout if not provided)")
	delay := flag.Bool("y", false, "")
	info := flag.Bool("info", false, "print info about the given TLE")
	config := flag.Bool("config", false, "use configuration file")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s-%s (%s)\n", Program, Version, BuildTime)
		os.Exit(2)
	}

	if flag.NArg() == 0 {
		flag.Usage()
	}

	sources := flag.Args()
	if *info {
		const (
			row = "%d | %s | %s | %s | %12s | %d"
			tfmt = "2006-01-02 15:04:05"
		)
		t, err := fetchTLE(sources, s.Temp, s.Sid)
		if err != nil {
			log.Fatalln(err)
		}

		for _, i := range t.Infos(s.Period.Duration, s.Interval.Duration) {
			delta := i.Ends.Sub(i.Starts)
			c := delta / s.Interval.Duration
			fmt.Printf(row, i.Sid, i.When.Format(tfmt), i.Starts.Format(tfmt), i.Ends.Format(tfmt), delta, c)
			fmt.Println()
		}
		return
	}

	if *config {
		if err := s.Update(flag.Arg(0)); err != nil {
			log.Fatalln(err)
		}
		sources = []string{s.Source}
	}
	s.Area.Syst = s.Print.Syst

	log.Printf("%s-%s (build: %s)", Program, Version, BuildTime)
	log.Printf("settings: trajectory duration %s", s.Period.Duration)
	log.Printf("settings: trajectory interval %s", s.Interval.Duration)
	log.Printf("settings: satellite identifier %d", s.Sid)
	log.Printf("settings: crossing area %s", s.Area.String())
	log.Printf("settings: latlon system %s", s.Print.Syst)
	t, err := fetchTLE(sources, s.Temp, s.Sid)
	if err != nil {
		log.Fatalln(err)
	}
	rs, err := t.Predict(s.Period.Duration, s.Interval.Duration, &s.Area, *delay)
	if err != nil {
		log.Fatalln(err)
	}
	var w io.Writer
	digest := md5.New()
	switch f, err := os.Create(s.File); {
	case err == nil:
		defer f.Close()
		w = io.MultiWriter(f, digest)
	case err != nil && s.File == "":
		w = io.MultiWriter(os.Stdout, digest)
	default:
		log.Fatalln(err)
	}
	m, err := s.Print.Print(w, rs)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("%d TLE used", m.TLE)
	log.Printf("%d positions predicted", m.Points)
	log.Printf("%d eclipses during trajectory", m.Eclipse)
	log.Printf("%d crossing during trajectory (%s)", m.Crossing, s.Area.String())

	log.Printf("md5: %x", digest.Sum(nil))
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

func transform(p *celest.Point, syst string) *celest.Point {
	switch strings.ToLower(syst) {
	default:
		g := p.Classic()
		return &g
	case "geocentric":
		g := p.Geocentric()
		return &g
	case "geodetic", "geodesic":
		g := p.Geodetic()
		return &g
	case "teme", "eci":
		return p
	}
}
