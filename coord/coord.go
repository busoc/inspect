package coord

import (
	"math"
	"time"
)

const Tolerance = 0.00000001

const (
	rad2deg = 180 / math.Pi
	deg2rad = math.Pi / 180
)

const (
	eex         = 0.006694385 //earth excentricity squared (see vallado for reference)
	flattening  = 0.003352813178
	earthRadius = 6378.1363
)

func Teme2ECEF(w time.Time, x, y, z float64) (float64, float64, float64) {
	return 0, 0, 0
}

func GeodeticFromECEF(x, y, z float64) (float64, float64, float64) {
	lat, lon, alt := ecef2Geodetic(x, y, z)
	return lat * rad2deg, lon * rad2deg, alt
}

func GeodeticToECEF(lat, lon, alt float64) (float64, float64, float64) {
	alt *= 1000
	lat *= deg2rad
	lon *= deg2rad

	sin := math.Sin(lat) * math.Sin(lat)
	n := (earthRadius * 1000) * math.Pow(1-flattening*(2-flattening)*sin, -0.5)

	x := (n + alt) * math.Cos(lat) * math.Cos(lon)
	y := (n + alt) * math.Cos(lat) * math.Sin(lon)
	z := (n*(1-eex) + alt) * math.Sin(lat)

	return x / 1000, y / 1000, z / 1000
}

func GeocentricFromECEF(x, y, z float64) (float64, float64, float64) {
	lat, lon, _ := ecef2Geodetic(x, y, z)
	norm := math.Sqrt(x*x + y*y + z*z)
	return math.Atan((1-eex)*math.Tan(lat)) * rad2deg, lon * rad2deg, norm - earthRadius
}

func ecef2Geodetic(x, y, z float64) (float64, float64, float64) {
	norm := math.Sqrt(x*x + y*y + z*z)
	radius := math.Sqrt(x*x + y*y)

	lon := math.Atan2(y/earthRadius, x/earthRadius)
	delta := math.Asin(z / norm)

	lat := delta
	var alt float64
	for i := 0; i < 2; i++ {
		delta = lat
		sin := math.Sin(lat)
		c := earthRadius / math.Sqrt(1-(eex*sin*sin))

		lat = math.Atan((z + c*eex*sin) / radius)
		alt = (radius / math.Cos(lat)) - c
		if diff := math.Abs(delta - lat); diff <= Tolerance {
			break
		}
	}
	return lat, lon, alt
}
