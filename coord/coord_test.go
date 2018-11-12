package coord

import (
  "fmt"
)

func ExampleGeodeticFromECEF() {
  x, y, z := 6524.834, 6862.875, 6448.296
  lat, lon, alt := GeodeticFromECEF(x, y, z)
  fmt.Printf("lat: %8.6f°, lon: %7.4f°, alt: %6.2f km", lat, lon, alt)
  // Output:
  // lat: 34.352496°, lon: 46.4464°, alt: 5085.22 km
}

func ExampleGeocentricFromECEF() {
  x, y, z := 6524.834, 6862.875, 6448.296
  lat, lon, _ := GeocentricFromECEF(x, y, z)
  fmt.Printf("lat: %8.6f°, lon: %7.4f°", lat, lon)
  // Output:
  // lat: 34.173429°, lon: 46.4464°
}
