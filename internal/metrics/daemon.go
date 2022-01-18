package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/prometheus/common/model"
)

const (
	metricSource  = "metric-source"
	scrapeTimeout = 15 * time.Second
)

type TargetMetric struct {
	name             string
	interval         time.Duration
	urls             []string
	scrapers         []*HTTPScraper
	ctx              context.Context
	cancel           context.CancelFunc
	forcetrigger     chan bool
	lock             sync.RWMutex
	store            API
	latestSuccessRun time.Time
	allowList        SampleFilter
}

func NewTargetMetric(targetName string, interval time.Duration, urls []string, store API, allowList SampleFilter) *TargetMetric {
	return &TargetMetric{
		name:         targetName,
		interval:     interval,
		urls:         urls,
		forcetrigger: make(chan bool),
		store:        store,
		allowList:    allowList,
	}
}

func (tg *TargetMetric) ForceEvent() {
	tg.forcetrigger <- true
}

func (tg *TargetMetric) LatestSuccessRun() time.Time {
	tg.lock.Lock()
	defer tg.lock.Unlock()
	return tg.latestSuccessRun
}

func (tg *TargetMetric) Start() {
	log.Infof("started targetMetric for workload '%v' with urls '%+v'",
		tg.name, tg.urls)

	tg.lock.Lock()
	// if it was started early, stop it
	if tg.cancel != nil {
		tg.cancel()
	}

	tg.createScraper()
	tg.ctx, tg.cancel = context.WithCancel(context.Background())
	ticker := time.NewTicker(tg.interval)
	tg.lock.Unlock()

	getdata := func() {
		log.Debugf("ticker run for workload '%v'", tg.name)
		ctx, _ := context.WithTimeout(context.Background(), scrapeTimeout)
		data := tg.Run(ctx)
		err := tg.Store(data, map[string]string{metricSource: tg.name})
		if err != nil {
			log.Error("cannot store target information:", err)
		}
	}
	for {
		select {
		case <-tg.forcetrigger:
			getdata()
		case <-ticker.C:
			getdata()
		case <-tg.ctx.Done():
			log.Debugf("time ticker for workload '%v' stopped", tg.name)
			ticker.Stop()
			return
		}
	}
}

// Store given data to the TSDB
func (tg *TargetMetric) Store(data model.Vector, labels map[string]string) error {
	if tg.store == nil {
		return fmt.Errorf("store interface is not set")
	}
	err := tg.store.AddVector(data, labels)
	if err != nil {
		return err
	}

	tg.lock.Lock()
	tg.latestSuccessRun = time.Now()
	tg.lock.Unlock()

	return nil
}

func (tg *TargetMetric) IsStopped() bool {
	return tg.cancel == nil
}

func (tg *TargetMetric) Stop() {
	tg.lock.Lock()
	defer tg.lock.Unlock()
	if tg.cancel != nil {
		tg.cancel()
	}
	tg.cancel = nil
	return
}

func (tg *TargetMetric) createScraper() {
	scrapers := []*HTTPScraper{}
	for _, url := range tg.urls {
		scrape, err := NewHTTPScraper(url)
		if err != nil {
			log.Errorf("cannot start scraper for %v: %v", url, err)
		}
		scrapers = append(scrapers, scrape)
	}
	tg.scrapers = scrapers
}

func (tg *TargetMetric) Run(ctx context.Context) model.Vector {
	res := model.Vector{}
	for _, scrape := range tg.scrapers {
		data, err := scrape.Scrape(ctx)
		if err != nil {
			log.Errorf("cannot get metrics for workload '%v': %v", tg.name, err)
			continue
		}
		res = append(res, tg.allowList.Filter(data)...)
	}
	return res
}

//go:generate mockgen -package=metrics -destination=mock_daemon.go . MetricsDaemon
type MetricsDaemon interface {
	Start()
	AddTarget(targetName string, urls []string, interval time.Duration)
	AddFilteredTarget(targetName string, urls []string, interval time.Duration, allowList SampleFilter)
	DeleteTarget(targetName string)
}

type metricsDaemon struct {
	targets map[string]*TargetMetric
	lock    sync.RWMutex
	store   API
}

func NewMetricsDaemon(store API) *metricsDaemon {
	return &metricsDaemon{
		targets: map[string]*TargetMetric{},
		lock:    sync.RWMutex{},
		store:   store,
	}
}

// GetTargets returns a list of current workload targets
func (md *metricsDaemon) GetTargets() []string {
	md.lock.Lock()
	defer md.lock.Unlock()
	res := []string{}
	for key := range md.targets {
		res = append(res, key)
	}
	return res
}

// Start started the target daemon.
func (md *metricsDaemon) Start() {
	md.lock.Lock()
	defer md.lock.Unlock()
	for _, target := range md.targets {
		md.startTarget(target)
	}
	return
}

func (md *metricsDaemon) startTarget(target *TargetMetric) {
	go target.Start()
}

// AddFilteredTarget adds a metrics scraping target with filtering
func (md *metricsDaemon) AddFilteredTarget(targetName string, urls []string, interval time.Duration, allowList SampleFilter) {
	target := NewTargetMetric(targetName, interval, urls, md.store, allowList)
	log.Debugf("added target '%v' with the following urls: '%+v", targetName, urls)

	// stop if it is already present.
	md.DeleteTarget(targetName)

	md.lock.Lock()
	defer md.lock.Unlock()
	md.targets[targetName] = target
	md.startTarget(target)
}

// AddTarget adds a metrics scraping target
func (md *metricsDaemon) AddTarget(targetName string, urls []string, interval time.Duration) {
	md.AddFilteredTarget(targetName, urls, interval, &PermissiveAllowList{})
}

// DeleteTarget deletes a target from getting metrics.
func (md *metricsDaemon) DeleteTarget(targetName string) {
	md.lock.Lock()
	defer md.lock.Unlock()
	target, ok := md.targets[targetName]
	if !ok {
		// No metric was set here
		return
	}

	log.Debugf("delete target '%v'", targetName)
	delete(md.targets, targetName)
	target.Stop()
}
