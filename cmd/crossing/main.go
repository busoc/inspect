package main

import (
	"flag"
	"fmt"
	"io"
	"os"
  "time"

	"github.com/midbel/linewriter"
)

func main() {
	var (
		comma  = flag.Bool("csv", false, "csv")
		list   = flag.Bool("list", false, "list")
		lat    = flag.Float64("lat", 0, "latitude")
		lng    = flag.Float64("lng", 0, "longitude")
		mgn    = flag.Float64("margin", 10, "margin")
		night  = flag.Bool("night", false, "night")
		starts = flag.String("starts", "", "start time")
		ends   = flag.String("ends", "", "end time")
		config = flag.Bool("config", false, "use config file")
		label  = flag.String("label", "", "label")
    cross  = flag.Duration("duration", 0, "duration")
	)
	flag.Parse()

	var (
		accept Accepter
		files  []string
	)
	if *config {
		s, err := Configure(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		accept, err = s.Accept()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		*list = s.List
		*comma = s.Comma
		files = []string{s.File}
	} else {
		sq, err := NewSquare(*lat, *lng, *mgn)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		pd, err := NewPeriod(*starts, *ends)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		ec := Eclipse(*night)

		fmt.Fprintln(os.Stderr, pd.String())
		fmt.Fprintln(os.Stderr, sq.String())
		fmt.Fprintln(os.Stderr, ec.String())

		accept = NewFilter(*label, sq, pd, ec)
		files = flag.Args()
	}

	var iter func(*linewriter.Writer, <-chan Point, time.Duration, Accepter) error
	if *list {
		iter = asList
	} else {
		iter = asAggr
	}
	queue, err := ReadPoints(files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := iter(Line(*comma), queue, *cross, accept); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func asAggr(ws *linewriter.Writer, queue <-chan Point, rng time.Duration, accept Accepter) error {
	var (
		first Point
		last  Point
	)
	for pt := range queue {
		if ok, label := accept.Accept(pt); ok {
			first, last = pt, pt
			for pt := range queue {
				if ok, _ := accept.Accept(pt); !ok {
					break
				}
				last = pt
			}

			var (
				delta = last.When.Sub(first.When)
				dist  = last.Distance(first)
			)
      if rng > 0 && delta < rng {
        continue
      }
      if label != "" {
        ws.AppendString(label, 12, linewriter.AlignLeft)
      }
			for _, p := range []Point{first, last} {
				ws.AppendTime(p.When, "2006-01-02T15:04:05.00", linewriter.AlignLeft)
				ws.AppendFloat(p.Lat, 8, 3, linewriter.AlignRight|linewriter.Float)
				ws.AppendFloat(p.Lng, 8, 3, linewriter.AlignRight|linewriter.Float)
			}
			ws.AppendDuration(delta, 8, linewriter.AlignRight|linewriter.Second)
			ws.AppendFloat(dist, 8, 1, linewriter.AlignRight|linewriter.Float)

			if _, err := io.Copy(os.Stdout, ws); err != nil && err != io.EOF {
				return err
			}
		}
	}
	return nil
}

func asList(ws *linewriter.Writer, queue <-chan Point, _ time.Duration, accept Accepter) error {
	for pt := range queue {
		if ok, label := accept.Accept(pt); ok {
      if label != "" {
        ws.AppendString(label, 12, linewriter.AlignLeft)
      }
			ws.AppendTime(pt.When, "2006-01-02 15:05:04.00", linewriter.AlignLeft)
			ws.AppendFloat(pt.Alt, 8, 3, linewriter.AlignRight|linewriter.Float)
			ws.AppendFloat(pt.Lat, 8, 3, linewriter.AlignRight|linewriter.Float)
			ws.AppendFloat(pt.Lng, 8, 3, linewriter.AlignRight|linewriter.Float)

			if pt.Eclipse {
				ws.AppendString("night", 5, linewriter.AlignRight)
			} else {
				ws.AppendString("day", 5, linewriter.AlignRight)
			}
			if pt.Saa {
				ws.AppendString("saa", 3, linewriter.AlignRight)
			} else {
				ws.AppendString("-", 3, linewriter.AlignRight)
			}

			if _, err := io.Copy(os.Stdout, ws); err != nil && err != io.EOF {
				return err
			}
		}
	}
	return nil
}

func Line(comma bool) *linewriter.Writer {
	var opts []linewriter.Option
	if comma {
		opts = append(opts, linewriter.AsCSV(true))
	} else {
		opts = []linewriter.Option{
			linewriter.WithPadding([]byte(" ")),
			linewriter.WithSeparator([]byte("|")),
		}
	}
	return linewriter.NewWriter(8192, opts...)
}
