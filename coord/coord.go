package coord

import (
  "math"
)

const Tolerance = 0.00000001

const rad2deg = 180 / math.Pi

const (
  eex         = 0.006694385 //earth excentricity squared (see vallado for reference)
  earthRadius = 6378.1363
)

func Geodetic(x, y, z float64) (float64, float64, float64) {
  lat, lon, alt := ecef2Geodetic(x, y, z)
  return lat*rad2deg, lon * rad2deg, alt
}

func Geocentric(x, y, z float64) (float64, float64, float64) {
  lat, lon, alt := ecef2Geodetic(x, y, z)
  return math.Atan((1-eex)*math.Tan(lat))*rad2deg, lon * rad2deg, alt
}

func ecef2Geodetic(x, y, z float64) (float64, float64, float64) {
  norm := math.Sqrt(x * x + y * y + z * z)
	radius := math.Sqrt(x * x + y * y)

	lon := math.Atan2(y/earthRadius, x/earthRadius)
	delta := math.Asin(z/norm)

	lat := delta
  var hell float64
  for {
		delta = lat
		sin := math.Sin(lat)
		c := earthRadius / math.Sqrt(1 - (eex * sin * sin))
		lat = math.Atan((z + c * eex * sin) / radius)
		if diff := math.Abs(delta - lat); diff <= Tolerance {
      hell = (radius /math.Cos(lat)) - c
			break
		}
	}
  return lat, lon, hell
}
