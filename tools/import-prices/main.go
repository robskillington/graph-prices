package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	sampleName   = "price_v0"
	influxDBAddr = "http://localhost:8086"
	influxDBName = "db"
)

func main() {
	r := csv.NewReader(os.Stdin)

	anyNonAlphanumericRegexp := regexp.MustCompile("[^a-zA-Z0-9]+")

	var row, headers []string
	var err error
	m := make(map[string]string)
	buff := bytes.NewBuffer(nil)
	reader := bytes.NewReader(nil)
	for row, err = r.Read(); err == nil; row, err = r.Read() {
		if len(row) <= 1 {
			// Not a row
			continue
		}
		if len(headers) == 0 {
			headers = row
			continue
		}

		// Remove all prev entries
		for k := range m {
			delete(m, k)
		}
		// Insert new entries
		for i, v := range row {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			m[headers[i]] = v
		}

		fmt.Printf("record:\n")
		for k, v := range m {
			fmt.Printf("%v=%v\n", k, v)
		}
		fmt.Printf("\n")

		price, ok := m["Price"]
		if !ok {
			// skip no price
			continue
		}

		priceValue, err := strconv.ParseFloat(price, 64)
		if err != nil {
			panic(fmt.Errorf("cannot parse price: %v", err))
		}

		date, ok := m["Date"]
		if !ok {
			panic("missing date")
		}

		segments := strings.Split(date, "/")
		if len(segments) != 3 {
			panic("bad date")
		}

		var nums [3]int
		for i := 0; i < len(segments); i++ {
			nums[i], err = strconv.Atoi(segments[i])
			if err != nil {
				panic(fmt.Errorf("cannot convert date err: %v", err))
			}
		}

		d := time.Date(nums[2], time.Month(nums[1]), nums[0], 0, 0, 0, 0, time.Local)

		// Cleanup tag data
		delete(m, "Price")
		delete(m, "Date")
		for k, v := range m {
			m[k] = anyNonAlphanumericRegexp.ReplaceAllString(v, "_")
		}

		buff.Truncate(0)
		buff.WriteString(sampleName)
		for k, v := range m {
			buff.WriteRune(',')
			buff.WriteString(k)
			buff.WriteRune('=')
			buff.WriteString(v)
		}
		fmt.Fprintf(buff, " value=%f %d", priceValue, d.UnixNano())

		reader.Reset(buff.Bytes())
		fmt.Printf("writing line:\n%v\n\n", buff.String())

		params := new(url.Values)
		params.Add("db", influxDBName)
		url := fmt.Sprintf("%s/write?%s", influxDBAddr, params.Encode())
		resp, err := http.Post(url, "text/plain", buff)
		if err != nil {
			panic(fmt.Errorf("request err: %v", err))
		}
		if resp.StatusCode < 200 || resp.StatusCode > 204 {
			panic(fmt.Errorf("request unexpected status: %v", resp.Status))
		}
		resp.Body.Close()
	}
	if err != nil && err != io.EOF {
		panic(err)
	}
}
