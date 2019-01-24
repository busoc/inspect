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
	Version   = "1.1.0"
	BuildTime = "2019-01-24 08:40:00"
)

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
		os.Exit(0)
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
	BStar    float64  `toml:"bstar"`

	Print printer `toml:"format"`
}

func (s *Settings) Update(f string) error {
	r, err := os.Open(f)
	if err != nil {
		return checkError(err, nil)
	}
	defer r.Close()

	if err := toml.NewDecoder(r).Decode(s); err != nil {
		return badUsage("invalid configuration file")
	}
	return nil
}

func main() {
	s := Settings{
		Area:     SAA,
		Temp:     os.TempDir(),
		Sid:      DefaultSid,
		Period:   Duration{time.Hour * 72},
		Interval: Duration{time.Minute},
		BStar:    -0.001,
	}

	flag.Float64Var(&s.BStar, "bstar", s.BStar, "bstar max")
	flag.StringVar(&s.Print.Format, "f", "", "format")
	flag.StringVar(&s.Print.Syst, "c", "", "system")
	flag.BoolVar(&s.Print.Round, "360", false, "round")
	flag.BoolVar(&s.Print.DMS, "dms", false, "dms")
	flag.StringVar(&s.Temp, "t", s.Temp, "temp dir")
	flag.IntVar(&s.Sid, "s", s.Sid, "satellite number")
	flag.Var(&s.Area, "r", "saa area")
	flag.Var(&s.Period, "d", "time range")
	flag.Var(&s.Interval, "i", "time interval")
	flag.StringVar(&s.File, "w", "", "write trajectory to file (stdout if not provided)")
	base := flag.String("b", "", "base time")
	delay := flag.Bool("y", false, "")
	info := flag.Bool("info", false, "print info about the given TLE")
	config := flag.Bool("config", false, "use configuration file")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *version {
		fmt.Fprintf(os.Stderr, "%s-%s (%s)\n", Program, Version, BuildTime)
		return
	}

	if flag.NArg() == 0 {
		flag.Usage()
	}

	var bt time.Time
	switch b, err := time.Parse(time.RFC3339, *base); {
	case err == nil:
		bt = b
	case err != nil && (*base == "-" || *base == "epoch" || *base == "tle"):
	case err != nil && (*base == "" || *base == "now"):
		bt = time.Now()
	default:
		Exit(badUsage("base time invalid value"))
	}

	if *info {
		if err := printInfos(flag.Args(), &s); err != nil {
			Exit(err)
		}
		return
	}

	var sources []string
	if *config {
		if err := s.Update(flag.Arg(0)); err != nil {
			Exit(err)
		}
		sources = []string{s.Source}
	} else {
		sources = flag.Args()
	}
	s.Area.Syst = s.Print.Syst

	log.Printf("%s-%s (build: %s)", Program, Version, BuildTime)
	log.Printf("settings: trajectory duration %s", s.Period.Duration)
	log.Printf("settings: trajectory interval %s", s.Interval.Duration)
	log.Printf("settings: satellite identifier %d", s.Sid)
	log.Printf("settings: bstar-drag coefficient limit %.6f", s.BStar)
	log.Printf("settings: crossing area %s", s.Area.String())
	log.Printf("settings: latlon system %s", s.Print.Syst)

	t, err := fetchTLE(sources, s.Temp, s.Sid, s.BStar)
	if err != nil {
		Exit(checkError(err, nil))
	}
	t.Base = bt

	rs, err := t.Predict(s.Period.Duration, s.Interval.Duration, &s.Area, *delay)
	if err != nil {
		Exit(checkError(err, nil))
	}
	var w io.Writer
	digest := md5.New()
	switch f, err := os.Create(s.File); {
	case err == nil:
		defer func() {
			if i, err := f.Stat(); err == nil {
				log.Printf("file: %s (%s, %dKB)", s.File, i.ModTime().Format(time.RFC1123), i.Size()>>10)
			}
			f.Close()
		}()
		w = io.MultiWriter(f, digest)
	case err != nil && s.File == "":
		w = io.MultiWriter(os.Stdout, digest)
	default:
		Exit(checkError(err, nil))
	}
	m, err := s.Print.Print(w, rs, s)
	if err != nil {
		Exit(checkError(err, nil))
	}
	log.Printf("%d TLE used", m.TLE)
	log.Printf("%d positions predicted", m.Points)
	log.Printf("%d eclipses during trajectory", m.Eclipse)
	log.Printf("%d crossing during trajectory (%s)", m.Crossing, s.Area.String())

	log.Printf("md5: %x", digest.Sum(nil))
}

func printInfos(sources []string, s *Settings) error {
	const (
		row  = "%d | %s | %s | %s | %12s | %d"
		tfmt = "2006-01-02 15:04:05"
	)
	t, err := fetchTLE(sources, s.Temp, s.Sid, s.BStar)
	if err != nil {
		return err
	}
	for _, i := range t.Infos(s.Period.Duration, s.Interval.Duration) {
		delta := i.Ends.Sub(i.Starts)
		c := delta / s.Interval.Duration
		fmt.Printf(row, i.Sid, i.When.Format(tfmt), i.Starts.Format(tfmt), i.Ends.Format(tfmt), delta, c)
		fmt.Println()
	}
	return nil
}

func fetchTLE(ps []string, copydir string, sid int, bstar float64) (*celest.Trajectory, error) {
	if len(ps) == 0 {
		return nil, fmt.Errorf("no input files given")
	}
	var t celest.Trajectory
	digest := md5.New()
	if resp, err := http.Get(ps[0]); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fetchError(ps[0], resp.StatusCode)
		}
		var w io.Writer = digest
		suffix := "-" + time.Now().Format("20060102_150405")
		if err := os.MkdirAll(copydir, 0755); err != nil && !os.IsExist(err) {
			return nil, checkError(err, nil)
		}
		if f, err := os.Create(path.Join(copydir, path.Base(ps[0]+suffix))); err == nil {
			defer f.Close()
			w = io.MultiWriter(f, w)
		}
		if err := t.Scan(io.TeeReader(resp.Body, w), sid, bstar); err != nil {
			return nil, checkError(err, nil)
		}
		log.Printf("parsing TLE from %s done (md5: %x, last-modified: %s)", ps[0], digest.Sum(nil), resp.Header.Get("last-modified"))
	} else {
		for _, p := range ps {
			r, err := os.Open(p)
			if err != nil {
				return nil, checkError(err, nil)
			}
			if err := t.Scan(io.TeeReader(r, digest), sid, bstar); err != nil {
				return nil, checkError(err, nil)
			}
			s, err := r.Stat()
			if err != nil {
				return nil, checkError(err, nil)
			}
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
	case "geodetic":
		g := p.Geodetic()
		return &g
	case "teme", "eci":
		return p
	case "dublin":
		g := p.Dublin()
		return &g
	case "cnes":
		g := p.CNES()
		return &g
	}
}
