package metrics

import (
	"git.sr.ht/~spc/go-log"
	"time"
)

type Monitor struct {
	metricsStorage *Metrics
}

func NewMonitor(metricsStorage *Metrics) *Monitor {
	return &Monitor{metricsStorage: metricsStorage}
}

func (m *Monitor) LogCurrentMetrics(onlyDelta bool) {
	lastTime := time.Now()
	for {
		time.Sleep(30 * time.Second)
		log.Info("Scanning metrics")
		m.stats()
		upperBound := time.Now()
		series, err := m.metricsStorage.GetMetricsFor(lastTime.Add(time.Second), upperBound)
		if err != nil {
			log.Error("cannot get metrics", err)
		}
		if onlyDelta {
			lastTime = upperBound
		}
		for {
			if !series.Next() {
				log.Infof("No more")
				break
			}
			s := series.At()
			log.Infof("Labels: %v", s)
			for i := s.Iterator(); i.Next(); {
				t, v := i.At()
				log.Infof("t: %v, v: %v", time.Unix(t/1000, 0), v)
			}
		}
	}
}

func (m *Monitor) stats() {
	for _, b := range m.metricsStorage.db.Blocks() {
		log.Infof("Block %v: %v - %v", b.Dir(), time.Unix(b.MinTime()/1000, 0), time.Unix(b.MaxTime()/1000, 0))
	}
	h := m.metricsStorage.db.Head()
	log.Infof("Head: %v - %v", time.Unix(h.MinTime()/1000, 0), time.Unix(h.MaxTime()/1000, 0))
}
