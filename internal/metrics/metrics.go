package metrics

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb"

	"github.com/jakub-dzon/k4e-operator/models"
)

const (
	millisInHour = 60 * 60 * 1000
	// DefaultRetentionDuration specifies 7 days data retention
	DefaultRetentionDuration = int64(7 * 24 * millisInHour)
	// DefaultMaxBytes specifies no block data size limit
	DefaultMaxBytes = int64(0)
)

type DataPoint struct {
	Time  int64
	Value float64
}
type Series struct {
	Labels     map[string]string
	DataPoints []DataPoint
}

//go:generate mockgen -package=metrics -destination=metrics_api_mock.go . API
type API interface {
	io.Closer
	Deregister() error
	GetMetricsForTimeRange(tMin time.Time, tMax time.Time) ([]Series, error)
	AddMetric(value float64, labelsMap map[string]string) error
	AddVector(data model.Vector, labelsMap map[string]string) error
}

type TSDB struct {
	appliedOptions *tsdb.Options
	db             *tsdb.DB
	dbLock         sync.RWMutex
}

func NewTSDB(dataDir string) (*TSDB, error) {
	metricsDir := path.Join(dataDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		log.Error(err)
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	opts := tsdb.DefaultOptions()

	opts.RetentionDuration = DefaultRetentionDuration
	opts.MaxBytes = DefaultMaxBytes

	db, err := tsdb.Open(metricsDir, nil, nil, opts, nil)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return &TSDB{
		db:             db,
		dbLock:         sync.RWMutex{},
		appliedOptions: opts,
	}, nil
}

func (t *TSDB) Deregister() error {
	metricsDir := t.db.Dir()
	err := t.Close()
	if err != nil {
		log.Error(err)
		return err
	}

	err = os.RemoveAll(metricsDir)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (t *TSDB) Close() error {
	t.dbLock.Lock()
	defer t.dbLock.Unlock()

	err := t.db.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func (t *TSDB) GetMetricsForTimeRange(tMin time.Time, tMax time.Time) ([]Series, error) {
	log.Tracef("Getting metrics for %v - %v", tMin, tMax)

	t.dbLock.RLock()
	defer t.dbLock.RUnlock()

	q, err := t.db.Querier(nil, toDbTime(tMin), toDbTime(tMax))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	defer func() {
		err = q.Close()
		if err != nil {
			log.Error(err)
		}
	}()

	matchers, err := getAllLabelsMatchers(q)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	seriesSet := q.Select(false, nil, matchers...)
	if seriesSet.Err() != nil {
		log.Error(seriesSet.Err())
		return nil, seriesSet.Err()
	}

	return toSeries(seriesSet), nil
}

func toSeries(seriesSet storage.SeriesSet) []Series {
	var allSeries []Series
	for {
		if !seriesSet.Next() {
			break
		}
		s := seriesSet.At()
		labelsMap := make(map[string]string)
		for _, label := range s.Labels() {
			labelsMap[label.Name] = label.Value
		}
		var dataPoints []DataPoint
		for i := s.Iterator(); i.Next(); {
			t, v := i.At()
			dataPoints = append(dataPoints, DataPoint{Time: t, Value: v})
		}
		allSeries = append(allSeries, Series{
			Labels:     labelsMap,
			DataPoints: dataPoints,
		})
	}
	return allSeries
}

func (t *TSDB) AddMetric(value float64, labelsMap map[string]string) error {
	t.dbLock.RLock()
	defer t.dbLock.RUnlock()

	appender := t.db.Appender(context.Background())
	_, err := appender.Append(0, labels.FromMap(labelsMap), toDbTime(time.Now()), value)
	if err != nil {
		if err := appender.Rollback(); err != nil {
			log.Error(err)
		}
		log.Error(err)
		return err
	}

	return appender.Commit()
}

func (t *TSDB) AddVector(data model.Vector, labelsMap map[string]string) error {
	var errors error
	for _, metric := range data {
		err := t.AddMetric(float64(metric.Value), mergeLabelsFromVector(metric.Metric, labelsMap))
		if err != nil {
			log.Errorf("cannot update metric: %v", err)
			errors = multierror.Append(errors, err)
		}
	}
	if errors != nil {
		return errors
	}
	return nil
}

func (t *TSDB) Init(config models.DeviceConfigurationMessage) error {
	return t.Update(config)
}

func (t *TSDB) Update(config models.DeviceConfigurationMessage) error {
	metricsConfiguration := config.Configuration.Metrics
	maxBytes := DefaultMaxBytes
	retentionDuration := DefaultRetentionDuration
	if metricsConfiguration != nil && metricsConfiguration.Retention != nil {
		maxBytes = int64(metricsConfiguration.Retention.MaxMib) * 1024 * 1024
		retentionDuration = int64(metricsConfiguration.Retention.MaxHours) * millisInHour
	}
	if retentionDuration == t.appliedOptions.RetentionDuration && maxBytes == t.appliedOptions.MaxBytes {
		return nil
	}
	log.Infof("Metrics retention changed. MaxBytes [%v -> %v] RetentionDuration: [%d -> %d]", t.appliedOptions.MaxBytes, maxBytes, t.appliedOptions.RetentionDuration, retentionDuration)

	t.dbLock.Lock()
	defer t.dbLock.Unlock()

	dir := t.db.Dir()

	err := t.db.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	t.appliedOptions.MaxBytes = maxBytes
	t.appliedOptions.RetentionDuration = retentionDuration
	db, err := tsdb.Open(dir, nil, nil, t.appliedOptions, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	t.db = db
	return nil
}

func getAllLabelsMatchers(q storage.Querier) ([]*labels.Matcher, error) {
	lbls, _, err := q.LabelNames()
	if err != nil {
		return nil, err
	}
	matchers := make([]*labels.Matcher, 0)
	for _, lbl := range lbls {
		matchers = append(matchers, labels.MustNewMatcher(labels.MatchRegexp, lbl, ".*"))
	}
	return matchers, nil
}

func toDbTime(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

func mergeLabelsFromVector(data model.Metric, labels map[string]string) map[string]string {
	res := map[string]string{}
	for k, v := range labels {
		res[k] = v
	}

	for k, v := range data {
		res[string(k)] = string(v)
	}
	return res
}
