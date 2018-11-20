package celest

import (
	"fmt"
	"math"
	"time"

	"github.com/busoc/celest/coord"
	"github.com/busoc/celest/sgp"
)

const (
	row1 = "%1d %5d%1s %8s %2d%12f %10f %6f%2d %6f%2d %1d %5s"
	row2 = "%d %5d %8f %8f %7f %8f %8f %11f%5d%1s"
)

type Result struct {
	TLE    []string
	Points []*Point
}

type Point struct {
	When  time.Time `json:"dtstamp" xml:"dtstamp"`
	Epoch float64   `json:"jd" xml:"jd"`

	// Satellite position
	Lat float64 `json:"lat" xml:"lat"`
	Lon float64 `json:"lon" xml:"lon"`
	Alt float64 `json:"alt" xml:"alt"`

	// SAA and Eclipse crossing
	Saa     bool `json:"crossing" xml:"crossing"`
	Partial bool `json:"-" xml:"-"`
	Total   bool `json:"eclipse" xml:"eclipse"`

	converted bool
}

func (p Point) MJD() float64 {
	return p.Epoch - deltaCnesJD
}

func (p Point) Classic() Point {
	if p.converted {
		return p
	}
	n := p
	n.Lat, n.Lon, n.Alt = ConvertTEME(p.When, []float64{n.Lat, n.Lon, n.Alt})
	return n
}

func (p Point) Geocentric() Point {
	if p.converted {
		return p
	}
	n := p
	n.converted = true
	n.Lat, n.Lon, n.Alt = coord.GeocentricFromECEF(p.toECEF())
	return n
}

func (p Point) Geodetic() Point {
	if p.converted {
		return p
	}
	n := p
	n.converted = true
	n.Lat, n.Lon, n.Alt = coord.GeodeticFromECEF(p.toECEF())
	return n
}

func (p Point) toECEF() (float64, float64, float64) {
	vs := []float64{p.Lat, p.Lon, p.Alt}
	// cs := ecefCoordinates(gstTime(p.When), vs)
	cs := ecefCoordinates(gstTimeBis(p.Epoch), vs)
	return cs[0], cs[1], cs[2]
}

type Shape interface {
	Contains(p Point) bool
}

type Element struct {
	Sid  int
	When time.Time
	JD   float64
	JDF  float64

	//Elements of row#1
	Year      int
	Doy       float64
	Mean1     float64
	Mean2     float64
	BStar     float64
	Ephemeris int

	//Elements of row#2
	Inclination  float64
	Ascension    float64
	Excentricity float64
	Perigee      float64
	Anomaly      float64
	Motion       float64
	Revolution   int

	TLE []string
}

func NewElement(row1, row2 string) (*Element, error) {
	var e Element
	if len(row1) != tleLen {
		return nil, InvalidLenError(len(row1))
	}
	if len(row2) != tleLen {
		return nil, InvalidLenError(len(row2))
	}
	if err := scanLine1(row1, &e); err != nil {
		return nil, err
	}
	if err := scanLine2(row2, &e); err != nil {
		return nil, err
	}
	e.TLE = []string{row1, row2}
	return &e, nil
}

func (e Element) Predict(p, s time.Duration, saa Shape) (*Result, error) {
	els := sgp.NewElsetrec()
	defer sgp.DeleteElsetrec(els)

	els.SetNumber(int64(e.Sid))
	els.SetYear(e.Year)
	els.SetDays(e.Doy)
	els.SetBstar(e.BStar)
	els.SetMean1(e.Mean1)
	els.SetMean2(e.Mean2)
	els.SetEphemeris(e.Ephemeris)

	els.SetJdsatepoch(e.JD)
	els.SetJdsatepochF(e.JDF)

	els.SetExcentricity(e.Excentricity)
	els.SetPerigee(e.Perigee)
	els.SetInclination(e.Inclination)
	els.SetAnomaly(e.Anomaly)
	els.SetMotion(e.Motion)
	els.SetAscension(e.Ascension)
	epoch := els.GetJdsatepoch() + els.GetJdsatepochF()
	wg84 := sgp.Gravconsttype(sgp.Wgs84)
	// TODO: move sgp4init in sgp package with func Init(e Elsetrec)
	if ok := sgp.Sgp4init(wg84, 'i', int(els.GetNumber()), epoch, els.GetBstar(), els.GetMean1(), els.GetMean2(), els.GetExcentricity(), els.GetPerigee(), els.GetInclination(), els.GetAnomaly(), els.GetMotion(), els.GetAscension(), els); !ok {
		return nil, fmt.Errorf("fail to initialize projection: %d", els.GetError())
	}

	var (
		ts []*Point
		js []float64
		es [][]float64
	)
	delta := s.Seconds() / time.Minute.Seconds()
	var when float64
	for elapsed := time.Duration(0); elapsed < p; elapsed += s {
		ps, _, err := sgp.SGP4(els, when)
		if err != nil {
			return nil, err
		}
		// TODO: wrap Invjday in sgp package with func Date(jd, jdf) time.Time
		var (
			year, month, day, hour, min int
			seconds                     float64
		)
		jd := els.GetJdsatepoch()
		jdf := els.GetJdsatepochF() + (when / minPerDays)
		if jdf < 0 {
			jd -= 1.0
			jdf += 1.0
		}

		sgp.Invjday(jd, jdf, &year, &month, &day, &hour, &min, &seconds)
		cs, ns := math.Modf(seconds)
		w := time.Date(year, time.Month(month), day, hour, min, int(cs), int(ns*1e9), time.UTC)

		t := Point{
			Lat:   ps[0],
			Lon:   ps[1],
			Alt:   ps[2],
			When:  w,
			Epoch: jd + jdf,
		}

		for i := range ps {
			ps[i] *= 1000
		}
		// TODO: compute eclipse on/off when knowing position of satellite
		es = append(es, ps)
		if saa != nil {
			t.Saa = saa.Contains(t.Geodetic())
		}
		ts = append(ts, &t)
		js = append(js, jd+jdf)
		// ws = append(ws, w)

		when += delta
	}
	fes, pes := eclipseStatus(es, js)
	for i := 0; i < len(ts); i++ {
		ts[i].Total = fes[i]
		ts[i].Partial = pes[i]
	}
	return &Result{TLE: e.TLE, Points: ts}, nil
}

func scanLine1(r string, e *Element) error {
	r1 := struct {
		Line      int
		Satellite int
		Class     string
		Label     string
		Year      int
		Doy       float64
		Mean1     float64
		Mean2     float64
		Mean2Exp  int
		BStar     float64
		BStarExp  int
		Ephemeris int
		Control   string
	}{}
	if _, err := fmt.Sscanf(r, row1, &r1.Line, &r1.Satellite, &r1.Class, &r1.Label, &r1.Year, &r1.Doy, &r1.Mean1, &r1.Mean2, &r1.Mean2Exp, &r1.BStar, &r1.BStarExp, &r1.Ephemeris, &r1.Control); err != nil || r1.Line != 1 {
		return fmt.Errorf("fail to scan row#1: %s", err)
	}
	mean1 := r1.Mean1 / (xpdotp * minPerDays)
	mean2 := ((r1.Mean2 / 100000) * math.Pow10(r1.Mean2Exp)) / (xpdotp * minPerDays * minPerDays)
	bstar := (r1.BStar / 100000) * math.Pow10(r1.BStarExp)

	e.Sid = r1.Satellite
	e.Year = r1.Year
	e.Doy = r1.Doy
	e.Mean1 = mean1
	e.Mean2 = mean2
	e.BStar = bstar
	e.Ephemeris = r1.Ephemeris

	var (
		month, day, hour, min int
		seconds               float64
	)
	if r1.Year < YPivot {
		r1.Year += Y2000
	} else {
		r1.Year += Y1900
	}
	sgp.Days2mdhms(r1.Year, r1.Doy, &month, &day, &hour, &min, &seconds)
	sgp.Jday(r1.Year, month, day, hour, min, seconds, &e.JD, &e.JDF)

	e.When = time.Date(r1.Year, time.Month(month), day, hour, min, int(seconds), 0, time.UTC)

	return nil
}

func scanLine2(r string, e *Element) error {
	r2 := struct {
		Line         int
		Satellite    int
		Inclination  float64
		Ascension    float64
		Excentricity float64
		Perigee      float64
		Anomaly      float64
		Motion       float64
		Revolution   int
		Control      string
	}{}

	if _, err := fmt.Sscanf(r, row2, &r2.Line, &r2.Satellite, &r2.Inclination, &r2.Ascension, &r2.Excentricity, &r2.Perigee, &r2.Anomaly, &r2.Motion, &r2.Revolution, &r2.Control); err != nil || r2.Line != 2 {
		return fmt.Errorf("fail to scan row#2: %s", err)
	}
	e.Inclination = r2.Inclination * deg2rad
	e.Ascension = r2.Ascension * deg2rad
	e.Excentricity = r2.Excentricity / 10000000
	e.Perigee = r2.Perigee * deg2rad
	e.Anomaly = r2.Anomaly * deg2rad
	e.Motion = r2.Motion / xpdotp

	return nil
}
