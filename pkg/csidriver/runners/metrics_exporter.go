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

package runners

import (
	"context"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"strings"
)

const (
	Subsystem        = "local"
	metricsNamespace = "carina"
)

type VolumeGroupMetrics struct {
	usedBytes  uint64
	totalBytes uint64
	vgName     string
}

type LogicVolumeMetrics struct {
	usedBytes  float64
	totalBytes uint64
	lvName     string
}

type metricsExporter struct {
	dm            *deviceManager.DeviceManager
	vgUsedBytes   *prometheus.GaugeVec
	vgTotalBytes  *prometheus.GaugeVec
	lvUsedBytes   *prometheus.GaugeVec
	lvTotalBytes  *prometheus.GaugeVec
	updateChannel chan *deviceManager.VolumeEvent
}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(dm *deviceManager.DeviceManager) manager.Runnable {
	vgUsedBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   Subsystem,
		Name:        "vg_used_bytes",
		Help:        "LVM VG used bytes",
		ConstLabels: prometheus.Labels{"nodename": dm.NodeName},
	}, []string{"vgname"})

	vgTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   Subsystem,
		Name:        "vg_total_bytes",
		Help:        "LVM VG total bytes",
		ConstLabels: prometheus.Labels{"nodename": dm.NodeName},
	}, []string{"vgname"})

	lvTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   Subsystem,
		Name:        "lv_total_bytes",
		Help:        "LVM logic Volume total bytes",
		ConstLabels: prometheus.Labels{"nodename": dm.NodeName},
	}, []string{"lvname"})

	lvUsedBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   Subsystem,
		Name:        "lv_used_bytes",
		Help:        "LVM logic volume used bytes",
		ConstLabels: prometheus.Labels{"nodename": dm.NodeName},
	}, []string{"lvname"})

	metrics.Registry.MustRegister(vgTotalBytes)
	metrics.Registry.MustRegister(vgUsedBytes)
	metrics.Registry.MustRegister(lvTotalBytes)
	metrics.Registry.MustRegister(lvUsedBytes)

	return &metricsExporter{
		dm:            dm,
		vgUsedBytes:   vgUsedBytes,
		vgTotalBytes:  vgTotalBytes,
		lvUsedBytes:   lvUsedBytes,
		lvTotalBytes:  lvTotalBytes,
		updateChannel: make(chan *deviceManager.VolumeEvent, 500), // Buffer up to 500 statuses
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ctx context.Context) error {
	m.dm.Cache.WaitForCacheSync(context.Background())

	log.Infof("Starting metricsExporter")
	defer log.Infof("Shutting down metricsExporter")
	defer close(m.updateChannel)

	// register volume update notice chan
	m.dm.RegisterNoticeChan(m.updateChannel)

	vgCh := make(chan VolumeGroupMetrics)
	lvCh := make(chan LogicVolumeMetrics)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case vg := <-vgCh:
				m.vgUsedBytes.WithLabelValues(vg.vgName).Set(float64(vg.usedBytes))
				m.vgTotalBytes.WithLabelValues(vg.vgName).Set(float64(vg.totalBytes))
			case lv := <-lvCh:
				m.lvUsedBytes.WithLabelValues(lv.lvName).Set(lv.usedBytes)
				m.lvTotalBytes.WithLabelValues(lv.lvName).Set(float64(lv.totalBytes))
			}
		}
	}()

	for {
		select {
		case ve := <-m.updateChannel:
			log.Infof("Update metric, trigger: %s, trigger at: %v", ve.Trigger, ve.TriggerAt.Format("2006-01-02 15:04:05.000000000"))

			diskSelectGroup := m.dm.GetNodeDiskSelectGroup()
			vgList, err := m.dm.VolumeManager.GetCurrentVgStruct()

			if err == nil && len(vgList) > 0 {
				for _, vg := range vgList {
					if _, ok := diskSelectGroup[vg.VGName]; !ok {
						continue
					}
					vgCh <- VolumeGroupMetrics{
						usedBytes:  vg.VGSize - vg.VGFree,
						totalBytes: vg.VGSize,
						vgName:     vg.VGName,
					}
				}
			}

			lvs, err := m.dm.VolumeManager.VolumeList("", "")

			if err == nil && len(lvs) > 0 {
				for _, lv := range lvs {
					if !strings.HasPrefix(lv.LVName, utils.VolumePrefix) {
						continue
					}
					if _, ok := diskSelectGroup[lv.VGName]; !ok {
						continue
					}
					lvCh <- LogicVolumeMetrics{
						lvName:     lv.LVName,
						totalBytes: lv.LVSize,
						usedBytes:  float64(lv.LVSize) * lv.DataPercent,
					}
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}
