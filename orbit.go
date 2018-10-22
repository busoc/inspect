package main

import (
	"math"
	"time"
)

const (
	earthRadius = 6378.136 * 1000
	sunRadius   = 6.96033e8
)

const (
	rad2deg = math.Pi * 180.0
	deg2rad = math.Pi / 180.0
	xpdotp  = minPerDays / (2.0 * math.Pi)
)

const (
	jd2mjd = 2400000.5
	jdByMil = 36525.0
)

const Axis = 3

func gstTime(t time.Time) float64 {
	_, _, cjd := mjdTime(t)
	h, m, s := float64(t.Hour())*secPerHours, float64(t.Minute())*secPerMins, float64(t.Second())

	gha := 23925.836 + 8640184*cjd + 0.092*cjd*cjd + (h + m + s)
	gst := gha * 360.0 / secPerDays
	gst -= math.Floor(gst/360.0) * 360.0

	return gst / 180.0 * math.Pi
}

func mjdTime(t time.Time) (float64, float64, float64) {
	y, m, d := float64(t.Year()), float64(t.Month()), float64(t.Day())
	h, i, s, ms := float64(t.Hour()), float64(t.Minute()), float64(t.Second()), float64(t.Nanosecond())/1000

	f := ((ms / math.Pow10(9)) + s + (i * secPerMins) + (h * secPerHours)) / secPerDays
	c := math.Trunc((m - 14) / 12)

	jd := d - 32075 + math.Trunc(1461*(y+4800+c)/4)
	jd += math.Trunc(367 * (m - 2 - c*12) / 12)
	jd -= math.Trunc(3 * (math.Trunc(y+4900+c) / 100) / 4)
	jd += f - 0.5

	return jd, jd - jd2mjd, (jd - 2415020) / jdByMil
}

func Convert(t time.Time, teme []float64) (float64, float64, float64) {
	ts := make([]float64, Axis)
	for i := range teme {
		ts[i] = teme[i] * 1000
	}
	rs := ecefCoordinates(t, ts)
	var norm float64
	for i := range rs {
		norm += rs[i] * rs[i]
	}
	norm = math.Sqrt(norm)

	lat := math.Asin(rs[2]/norm) / math.Pi * 180
	lon := math.Atan2(rs[1], rs[0]) / math.Pi * 180

	return lat, lon, norm - earthRadius
}

func ecefCoordinates(t time.Time, teme []float64) []float64 {
	gst := gstTime(t)

	cos, sin := math.Cos(gst), math.Sin(gst)
	x, y := teme[0], teme[1]

	rs := make([]float64, Axis)
	rs[0] = cos*x + sin*y
	rs[1] = -sin*x + cos*y
	rs[2] = teme[2]

	return rs
}

func sunPosition(ws []time.Time) [][]float64 {
	const (
		omega   = 282.9400
		epsilon = 23.43929111 / 180 * math.Pi
	)
	ps := make([][]float64, len(ws))
	for i := range ws {
		jd, _, _ := mjdTime(ws[i])
		cjd := (jd - 2451545.0) / jdByMil
		m := 357.5256 + (35999.049 * cjd)
		ecliptic := omega + m + (6892 / secPerHours * math.Sin(m/rad2deg)) + (72 / secPerHours * math.Sin(2*m/rad2deg))
		distance := (149.619 - (2.499 * math.Cos(m/rad2deg)) - (0.021 * math.Cos(2*m/rad2deg))) * math.Pow10(9)

		lat := distance * math.Cos(ecliptic/180*math.Pi)
		lon := distance * math.Sin(ecliptic/180*math.Pi) * math.Cos(epsilon)
		alt := distance * math.Sin(ecliptic/180*math.Pi) * math.Sin(epsilon)
		ps[i] = []float64{lat, lon, alt}
	}
	return ps
}

func eclipseStatus(ps [][]float64, ws []time.Time) ([]bool, []bool) {
	sun := sunPosition(ws)
	t1 := make([][]float64, len(ps))
	t2 := make([][]float64, len(ps))
	for i := 0; i < len(ps); i++ {
		t1[i] = make([]float64, Axis)
		t1[i][0] = sun[i][0] - ps[i][0]
		t1[i][1] = sun[i][1] - ps[i][1]
		t1[i][2] = sun[i][2] - ps[i][2]

		t2[i] = make([]float64, Axis)
		t2[i][0] = -ps[i][0]
		t2[i][1] = -ps[i][1]
		t2[i][2] = -ps[i][2]
	}
	direction := normalizeArray(t1)
	nadir := normalizeArray(t2)

	var earthSunAngles, earthAngles, sunAngles []float64
	for _, d := range dotProductArray(direction, nadir) {
		earthSunAngles = append(earthSunAngles, math.Acos(d))
	}
	for _, d := range normsArray(ps) {
		earthAngles = append(earthAngles, math.Asin(earthRadius/d))
	}
	for _, d := range normsArray(sun) {
		sunAngles = append(sunAngles, math.Asin(sunRadius/d))
	}
	fes := make([]bool, len(ps))
	pes := make([]bool, len(ps))
	for i := range fes {
		fa := earthSunAngles[i] < math.Abs(earthAngles[i]-sunAngles[i])
		fb := earthAngles[i] > sunAngles[i]
		fes[i] = fa && fb

		pa := earthSunAngles[i] > math.Abs(earthAngles[i]-sunAngles[i])
		pb := earthAngles[i]+sunAngles[i] > earthSunAngles[i]
		pes[i] = pa && pb
	}

	return fes, pes
}

func normsArray(ps [][]float64) []float64 {
	ns := make([]float64, len(ps))
	for i := 0; i < len(ns); i++ {
		x, y, z := ps[i][0], ps[i][1], ps[i][2]
		n := x*x + y*y + z*z
		ns[i] = math.Sqrt(n)
	}
	return ns
}

func normalizeArray(ps [][]float64) [][]float64 {
	norm := normsArray(ps)
	as := make([][]float64, len(ps))
	for i := 0; i < Axis; i++ {
		for j := 0; j < len(ps); j++ {
			as[j] = append(as[j], ps[j][i]/norm[j])
		}
	}
	return as
}

func dotProductArray(a, b [][]float64) []float64 {
	var ds []float64
	for i := 0; i < len(a); i++ {
		var n float64
		for j := 0; j < Axis; j++ {
			n += a[i][j] * b[i][j]
		}
		ds = append(ds, n)
	}
	return ds
}
