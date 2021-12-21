package metrics

import (
	"git.sr.ht/~spc/go-log"
	"math/rand"
	"time"
)

type Generator struct {
	metricsStorage *Metrics
}

func NewGenerator(metricsStorage *Metrics) *Generator {
	return &Generator{metricsStorage: metricsStorage}
}

func (g *Generator) GenerateRandom(metricName string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		err := g.metricsStorage.AddMetric(metricName, rand.Float64()*100)
		if err != nil {
			log.Error(err)
		}
	}
}
