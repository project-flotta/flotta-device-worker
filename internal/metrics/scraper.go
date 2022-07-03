package metrics

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

const (
	acceptHeader = `application/openmetrics-text; version=0.0.1,text/plain;version=0.0.4;q=0.5,*/*;q=0.1`
	UserAgent    = "flotta-device-worker"
)

type Scraper interface {
	Scrape(ctx context.Context) (model.Vector, error)
}

type HTTPScraper struct {
	client *http.Client
	req    *http.Request
}

func NewHTTPScraper(url string) (Scraper, error) {
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

	return decodeSamples(reader, resp.Header.Get("Content-Type"))

}

func (s *HTTPScraper) String() string {
	return fmt.Sprintf("HTTP scraper for URL: %s", s.req.RequestURI)
}

func decodeSamples(reader io.Reader, contentType string) (model.Vector, error) {
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

type ObjectScraper struct {
	gatherer prometheus.Gatherer
}

func NewObjectScraper(gather prometheus.Gatherer) Scraper {
	return &ObjectScraper{gatherer: gather}
}

func (o *ObjectScraper) Scrape(_ context.Context) (model.Vector, error) {
	metrics, err := o.gatherer.Gather()
	if err != nil {
		return nil, err
	}
	v := model.Vector{}
	for _, m := range metrics {
		metric := model.Metric{model.MetricNameLabel: model.LabelValue(*m.Name)}
		for _, item := range m.Metric {
			for _, labels := range item.Label {
				metric[model.LabelName(*labels.Name)] = model.LabelValue(*labels.Value)
			}
			v = append(v, &model.Sample{Metric: metric, Value: model.SampleValue(*item.Counter.Value), Timestamp: model.Now()})
		}
	}
	return v, nil
}

func (o *ObjectScraper) String() string {
	return "Prometheus object scraper"
}
