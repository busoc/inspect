package main

import (
  "io"
  "os"
  "log"
  "encoding/csv"
  "flag"
  "strconv"
  "time"
  "math"

  "github.com/busoc/celest"
)

func init() {
  log.SetOutput(os.Stdout)
  log.SetFlags(0)
}

const Leap = time.Second*18

func main() {
  coord := flag.String("m", "", "coordinate system")
  fill := flag.Bool("f", false, "fill missing row(s)")
  round := flag.Bool("360", false, "360")
  flag.Parse()

  r := csv.NewReader(os.Stdin)
  var prev *celest.Point
  for i := 0; ; i++ {
    rs, err := r.Read()
    if rs == nil && err == io.EOF {
      break
    }
    if err != nil && err != io.EOF {
      log.Fatalln(err)
    }
    p, err := parsePoint(rs, *coord, *round)
    if err != nil {
      log.Fatalln(err)
    }
    if prev != nil && p.When.Equal(prev.When) {
      i--
      continue
    }
    if *fill && prev != nil && p.When.Sub(prev.When) > time.Second {
      w := prev.When.Add(time.Second)
      for w.Before(p.When) && p.When.Sub(w) >= time.Second {
        log.Printf("%6d | %s | %12.6f | %12.5f | %12.5f | %12.5f", i, w.Format("2006-01-02T15:04:05.000000"), prev.MJD(), prev.Alt, prev.Lat, prev.Lon)
        w = w.Add(time.Second)
        i++
      }
    }
    log.Printf("%6d | %s | %12.6f | %12.5f | %12.5f | %12.5f", i, p.When.Format("2006-01-02T15:04:05.000000"), p.MJD(), p.Alt, p.Lat, p.Lon)
    prev = p
  }
}

func parsePoint(rs []string, t string, round bool) (*celest.Point, error) {
  var (
    p celest.Point
    err error
  )
  if p.When, err = time.Parse(time.RFC3339, rs[1]); err != nil {
    return nil, err
  } else {
    p.When = p.When.Add(-Leap).UTC()
    p.Epoch = celest.JD(p.When)
  }
  if p.Alt, err = strconv.ParseFloat(rs[2], 64); err != nil {
    return nil, err
  }
  if p.Lat, err = strconv.ParseFloat(rs[3], 64); err != nil {
    return nil, err
  }
  if p.Lon, err = strconv.ParseFloat(rs[4], 64); err != nil {
    return nil, err
  }

  switch t {
  case "geodetic":
    p = p.Geodetic()
  case "geocentric":
    p = p.Geocentric()
  case "teme", "eci":
    round = false
  default:
    p = p.Classic()
  }
  if round {
    p.Lon = math.Mod(p.Lon+360, 360)
  }
  p.When = p.When.Truncate(time.Second)
  return &p, nil
}
