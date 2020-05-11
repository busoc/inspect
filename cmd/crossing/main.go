package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/midbel/linewriter"
)

func main() {
	var (
		comma  = flag.Bool("csv", false, "csv")
		// list   = flag.Bool("list", false, "list")
		lat    = flag.Float64("lat", 0, "latitude")
		lng    = flag.Float64("lng", 0, "longitude")
		mgn    = flag.Float64("margin", 10, "margin")
		night  = flag.Bool("night", false, "night")
		starts = flag.String("starts", "", "start time")
		ends   = flag.String("ends", "", "end time")
		config = flag.Bool("config", false, "use config file")
		label  = flag.String("label", "", "label")
	)
	flag.Parse()

	var (
		paths []Path
		err   error
	)
	if *config {
		s, err := Configure(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		*comma = s.Comma

		paths, err = s.Paths()
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

		paths, err = ReadPaths(flag.Args(), NewFilter(*label, sq, pd, ec))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	printPaths(Line(*comma), paths)
}

func printPaths(ws *linewriter.Writer, paths []Path) error {
	sort.Slice(paths, func(i, j int) bool {
		return paths[i].Less(paths[j])
	})
	for _, p := range paths {
		var (
			delta = p.Delta()
			dist = p.Distance()
		)
		if p.Label != "" {
			ws.AppendString(p.Label, 12, linewriter.AlignLeft)
		}
		for _, p := range []Point{p.First, p.Last} {
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
