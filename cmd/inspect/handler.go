package main

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"

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
		_, err = e.Predict(n.Period.Duration, n.Interval.Duration, &n.Area)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch accept := r.Header.Get("accept"); accept {
		case "application/json":
			w.Header().Set("content-type", accept)
		case "application/xml":
			w.Header().Set("content-type", accept)
		case "txt/csv":
			w.Header().Set("content-type", accept)
		default:
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
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
		rs := bufio.NewReader(r.Body)
		vs := make([]string, 2)
		for i := 0; i < len(vs); i++ {
			vs[i], err = rs.ReadString('\n')
			if err != nil {
				break
			}
		}
		c.Row1, c.Row2 = vs[0], vs[1]
	default:
		return nil, fmt.Errorf("unsupported content-type")
	}
	if err != nil {
		return nil, err
	}
	return celest.NewElement(c.Row1, c.Row2)
}
