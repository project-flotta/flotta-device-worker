package metrics

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"io"
	"net/http"
)

const (
	acceptHeader = `application/openmetrics-text; version=0.0.1,text/plain;version=0.0.4;q=0.5,*/*;q=0.1`
)

var (
	UserAgent = fmt.Sprintf("flotta-device-worker")
)

type Scraper interface {
	Scrape(ctx context.Context) (model.Vector, error)
}

type HTTPScraper struct {
	client *http.Client
	req    *http.Request
}

func NewHTTPScraper(url string) (*HTTPScraper, error) {
	client := http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", acceptHeader)
	req.Header.Add("Accept-Encoding", "gzip")
	req.Header.Set("User-Agent", UserAgent)

	return &HTTPScraper{
		client: &client,
		req:    req,
	}, nil
}

func (s *HTTPScraper) Scrape(ctx context.Context) (model.Vector, error) {
	resp, err := s.client.Do(s.req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("server returned HTTP status %s", resp.Status)
	}

	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzipReader, err := gzip.NewReader(
			bufio.NewReader(resp.Body),
		)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = gzipReader.Close()
		}()
		reader = gzipReader
	default:
		reader = resp.Body
	}

	contentType := resp.Header.Get("Content-Type")
	decoder := expfmt.SampleDecoder{
		Dec:  expfmt.NewDecoder(reader, expfmt.Format(contentType)),
		Opts: &expfmt.DecodeOptions{Timestamp: model.Now()},
	}

	var allSamples model.Vector
	for {
		var mfSamples model.Vector
		err := decoder.Decode(&mfSamples)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		allSamples = append(allSamples, mfSamples...)
	}

	return allSamples, nil
}
