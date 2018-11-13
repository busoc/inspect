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
}
