package metrics

import (
	"context"
	"fmt"
	"git.sr.ht/~spc/go-log"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"
	"os"
	"path"
	"time"
)

type Metrics struct {
	db *tsdb.DB
}

func NewMetrics(dataDir string) (*Metrics, error) {
	metricsDir := path.Join(dataDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	db, err := tsdb.Open(metricsDir, nil, nil, nil, nil)
	return &Metrics{db: db}, err
}

func (m *Metrics) Deregister() error {
	return m.db.Close()
}

func (m *Metrics) GetMetricsFor(tMin time.Time, tMax time.Time) (storage.SeriesSet, error) {
	log.Infof("Getting metrics for %v - %v", tMin, tMax)
	q, err := m.db.Querier(nil, tMin.UnixNano(), tMax.UnixNano())
	if err != nil {
		return nil, err
	}

	lbls, _, err := q.LabelNames()
	if err != nil {
		return nil, err
	}
	matchers := make([]*labels.Matcher, 0)
	for _, lbl := range lbls {
		matchers = append(matchers, labels.MustNewMatcher(labels.MatchRegexp, lbl, ".*"))
	}
	seriesSet := q.Select(false, nil, matchers...)
	//seriesSet := q.Select(false, nil)
	log.Info(seriesSet.Err())
	defer func() {
		err = q.Close()
		if err != nil {
			log.Error(err)
		}
	}()
	return seriesSet, nil
}

func (m *Metrics) AddMetric(name string, value float64) error {
	appender := m.db.Appender(context.Background())
	_, err := appender.Append(0, labels.FromStrings("name", name), time.Now().UnixNano(), value)
	if err != nil {
		return err
	}

	return appender.Commit()
}
