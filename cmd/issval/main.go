package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/busoc/celest"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)

	keep := flag.Bool("k", false, "keep xyz coordinates")
	flag.Parse()
	for p := range fetchPoints(flag.Args()) {
		var saa, eclipse int
		if p.Saa {
			saa++
		}
		if p.Eclipse {
			eclipse++
		}
		p.When = p.When.Add(Delta)
		if !*keep {
			p.Lat, p.Lon, p.Alt = celest.ConvertTEME(p.When, []float64{p.Lat, p.Lon, p.Alt})
		}
		log.Printf("%s | %18.5f | %18.5f | %18.5f | %d | %d", p.When.Format("2006-01-02 15:04:05"), p.Alt/1000, p.Lat, p.Lon, eclipse, saa)
	}
}

const Leap = 18 * time.Second

const FeetTo = 3280.841

var (
	UNIX  = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
	GPS   = time.Date(1980, 1, 6, 0, 0, 0, 0, time.UTC)
	Delta = GPS.Sub(UNIX)
)

type Point struct {
	When time.Time

	Lat float64
	Lon float64
	Alt float64

	Saa     bool
	Eclipse bool
}

func fetchPoints(ps []string) <-chan *Point {
	q := make(chan *Point)
	go func() {
		defer close(q)
		for _, p := range ps {
			err := filepath.Walk(p, func(p string, i os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if i.IsDir() {
					return nil
				}
				r, err := os.Open(p)
				if err != nil {
					return err
				}
				defer r.Close()
				ps, err := readPoints(r)
				if err == nil {
					for _, p := range ps {
						q <- p
					}
				}
				return err
			})
			if err != nil {
				return
			}
		}
	}()
	return q
}

// BAD_GNC_Inert_(X|Y|Z)
var (
	UMIPosX = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x68} //0x0C0000008768
	UMIPosY = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x69} //0x0C0000008769
	UMIPosZ = []byte{0x0C, 0x00, 0x00, 0x00, 0x87, 0x6A} //0x0C000000876A
)

var (
	UMISaaStat = []byte{0x0C, 0x00, 0x00, 0x00, 0x8E, 0xEE} //0x0C0000008EEE
	UMISunStat = []byte{0x0C, 0x00, 0x00, 0x00, 0x8E, 0xEF} //0x0C0000008EEF
)

func readPoints(r io.Reader) ([]*Point, error) {
	s := bufio.NewScanner(r)
	s.Split(scanPackets)

	var (
		ps   []*Point
		curr *Point
	)
	for s.Scan() {
		bs := s.Bytes()
		if bs[0] != 2 {
			continue
		}
		coarse := binary.BigEndian.Uint32(bs[14:])
		fine := uint8(bs[18])

		w := readTime5(coarse, fine).Truncate(time.Second)
		switch xs := bs[5:11]; {
		case bytes.Equal(xs, UMIPosX):
			tmp := &Point{When: w}
			if curr != nil {
				tmp.Saa = curr.Saa
				tmp.Eclipse = curr.Eclipse
			}
			curr = tmp
			ps = append(ps, curr)
			curr.Lat = readFloat(bs[21:])
		case bytes.Equal(xs, UMIPosY) && curr != nil:
			curr.Lon = readFloat(bs[21:])
		case bytes.Equal(xs, UMIPosZ) && curr != nil:
			curr.Alt = readFloat(bs[21:])
		case bytes.Equal(xs, UMISaaStat) && curr != nil:
			v := bytes.Trim(bs[21:], "\x00")
			curr.Saa = string(v) == "IN"
		case bytes.Equal(xs, UMISunStat) && curr != nil:
			v := bytes.Trim(bs[21:], "\x00")
			curr.Eclipse = string(v) == "NIGHT"
		default:
			continue
		}
	}
	return ps, s.Err()
}

func readFloat(bs []byte) float64 {
	v := binary.BigEndian.Uint64(bs)
	return math.Float64frombits(v) / FeetTo
}

func scanPackets(bs []byte, ateof bool) (int, []byte, error) {
	if len(bs) < 4 {
		return 0, nil, nil
	}
	s := int(binary.LittleEndian.Uint32(bs[:4]))
	if s+4 >= len(bs) {
		return 0, nil, nil
	}
	vs := make([]byte, s)
	return copy(vs, bs[4:4+s]) + 4, vs, nil
}

func readTime5(coarse uint32, fine uint8) time.Time {
	t := time.Unix(int64(coarse), 0).UTC()

	fs := float64(fine) / 256.0 * 1000.0
	ms := time.Duration(fs) * time.Millisecond
	return t.Add(ms).UTC()
}
