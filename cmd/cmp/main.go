package main

import (
	"encoding/csv"
	"flag"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/busoc/celest"
)

const timeFormat = "2006-01-02T15:04:05.000000"

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
	flag.Parse()
	r, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatalln(err)
	}
	defer r.Close()

	rs := csv.NewReader(r)
	rs.Comment = '#'
	rs.Comma = ','
	rs.FieldsPerRecord = '7'
	if _, err := rs.Read(); err != nil {
		log.Fatalln(err)
	}
	for {
		row, err := rs.Read()
		if err == io.EOF && row == nil {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}
		w, err := time.Parse(timeFormat, row[0])
		if err != nil {
			log.Fatalln(err)
		}
		x, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			log.Fatalln("ECI X:", err)
		}
		y, err := strconv.ParseFloat(row[6], 64)
		if err != nil {
			log.Fatalln("ECI Y:", err)
		}
		z, err := strconv.ParseFloat(row[7], 64)
		if err != nil {
			log.Fatalln("ECI Z:", err)
		}

		lat, lon, alt := celest.ConvertTEME(w, []float64{x / 1000, y / 1000, z / 1000})
		log.Printf("%s | %18.5f | %18.5f | %18.5f", w.Format(timeFormat), alt/1000, lat, lon)
	}
}
