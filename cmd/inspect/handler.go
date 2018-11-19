package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/busoc/celest"
)

func Handle(s Settings) http.Handler {
	f := func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		n := s
		e, err := elementFromRequest(r, &n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rs, err := e.Predict(n.Period.Duration, n.Interval.Duration, &n.Area)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var buffer bytes.Buffer
		accept := r.Header.Get("accept")
		switch accept {
		case "application/json":
			ps := make([]*celest.Point, len(rs.Points))
			for i := range ps {
				ps[i] = n.Print.transform(rs.Points[i])
			}
			err = json.NewEncoder(&buffer).Encode(ps)
		case "application/xml":
			ps := make([]*celest.Point, len(rs.Points))
			for i := range ps {
				ps[i] = n.Print.transform(rs.Points[i])
			}
			err = xml.NewEncoder(&buffer).Encode(rs.Points)
		case "text/csv":
			err = n.Print.printRow(csv.NewWriter(&buffer), rs)
		default:
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if buffer.Len() == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("content-type", accept)
		w.Header().Set("content-length", fmt.Sprint(buffer.Len()))
		io.Copy(w, &buffer)
	}
	return http.HandlerFunc(f)
}

func elementFromRequest(r *http.Request, s *Settings) (*celest.Element, error) {
	c := struct {
		Row1      string `json:"row1" xml:"tle>row1"`
		Row2      string `json:"row2" xml:"tle>row2"`
		*Settings `json:"settings" xml:"settings"`
	}{Settings: s}

	var err error
	switch ct := r.Header.Get("content-type"); ct {
	case "application/json":
		err = json.NewDecoder(r.Body).Decode(&c)
	case "application/xml":
		err = xml.NewDecoder(r.Body).Decode(&c)
	case "text/plain":
		// rs := bufio.NewReader(r.Body)
		vs := make([]string, 2)
		rs := bufio.NewScanner(r.Body)
		for i := 0; i < len(vs); i++ {
			rs.Scan()
			vs[i] = rs.Text()
			if err = rs.Err(); err != nil {
				break
			}
		}
		c.Row1, c.Row2 = vs[0], vs[1]
		q := r.URL.Query()
		if v := q.Get("period"); v != "" {
			s.Period.Duration, err = time.ParseDuration(v)
		}
		if v := q.Get("interval"); v != "" {
			s.Interval.Duration, err = time.ParseDuration(v)
		}
		s.Print.Syst = q.Get("frames")
	default:
		return nil, fmt.Errorf("unsupported content-type")
	}
	if err != nil {
		return nil, err
	}
	return celest.NewElement(c.Row1, c.Row2)
}
