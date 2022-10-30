/*
  Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package metrics

import (
	"errors"
	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils/log"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"sync"
	"time"
)

const (
	namespace       string = "carina"
	scrapeSubSystem string = "scrape"
)

var (
	// ErrNoData indicates the collector found no data to collect, but had no other error.
	ErrNoData   = errors.New("collector returned no data")
	nodeName    = os.Getenv("NODE_NAME")
	constLabels = prometheus.Labels{"nodename": nodeName}

	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, scrapeSubSystem, "collector_duration_seconds"),
		"carina_csi_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, scrapeSubSystem, "collector_success"),
		"carina_csi_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

type typedFactorDesc struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
}

func (d *typedFactorDesc) mustNewConstMetric(value float64, labels ...string) prometheus.Metric {
	return prometheus.MustNewConstMetric(d.desc, d.valueType, value, labels...)
}

// Collector is the interface a collector has to implement.
type Collector interface {
	Update(ch chan<- prometheus.Metric) error
	Name() string
}

// CarinaCollector implements the prometheus.Collector interface.
type CarinaCollector struct {
	collectors map[string]Collector
	dm         *deviceManager.DeviceManager
}

func NewCarinaCollector(dm *deviceManager.DeviceManager, lvService *k8s.LogicVolumeService) (*CarinaCollector, error) {
	collectors := make(map[string]Collector)

	vgStatsCollector, err := newVolumeGroupStatsCollector(dm)
	if err != nil {
		return nil, err
	}
	volumeStatsCollector, err := newVolumeStatsCollector(lvService)
	if err != nil {
		return nil, err
	}
	collectors[vgStatsCollector.Name()] = vgStatsCollector
	collectors[volumeStatsCollector.Name()] = volumeStatsCollector

	return &CarinaCollector{collectors: collectors, dm: dm}, nil
}

// Describe implements the prometheus.Collector interface.
func (c CarinaCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// Collect implements the prometheus.Collector interface.
func (c CarinaCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.collectors))
	for name, c := range c.collectors {
		go func(name string, c Collector) {
			execute(name, c, ch)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func execute(name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if IsNoDataError(err) {
			log.Debug("msg ", "collector returned no data ", "name ", name, "duration_seconds ", duration.Seconds(), "err ", err)
		} else {
			log.Debug("msg ", "collector failed ", "name ", name, "duration_seconds ", duration.Seconds(), "err ", err)
		}
		success = 0
	} else {
		log.Debug("msg ", "collector succeeded ", "name ", name, "duration_seconds ", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func IsNoDataError(err error) bool {
	return err == ErrNoData
}
