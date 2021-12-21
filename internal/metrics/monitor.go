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
		upperBound := time.Now()
		series, err := m.metricsStorage.GetMetricsFor(lastTime, upperBound)
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
				log.Infof("t: %v, v: %v", t, v)
			}
		}
	}
}
