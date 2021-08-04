/*
   Copyright @ 2021 fushaosong <fushaosong@beyondlet.com>.

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
	"github.com/bocloud/carina/pkg/devicemanager/volume"
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const metricsNamespace = "carina"

// Device Metrics
type DeviceMetrics struct {
	FreeBytes   uint64
	TotalBytes  uint64
	DeviceGroup string
}

// volume Metrics
type VolumeMetrics struct {
	Volume     string
	TotalBytes uint64
	UsedBytes  float64
}

type metricsExporter struct {
	nodeName         string
	volume           volume.LocalVolume
	vgFreeBytes      *prometheus.GaugeVec
	vgTotalBytes     *prometheus.GaugeVec
	volumeTotalBytes *prometheus.GaugeVec
	volumeUsedBytes  *prometheus.GaugeVec
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func NewMetricsExporter(nodeName string, volume volume.LocalVolume) manager.Runnable {
	vgFreeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "devicegroup",
		Name:        "vg_free_bytes",
		Help:        "LVM VG free bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_group"})

	vgTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "devicegroup",
		Name:        "vg_total_bytes",
		Help:        "LVM VG total bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"device_group"})

	volumeTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volume",
		Name:        "volume_total_bytes",
		Help:        "LVM Volume total bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"volume"})

	volumeUsedBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volume",
		Name:        "volume_used_bytes",
		Help:        "LVM volume used bytes",
		ConstLabels: prometheus.Labels{"node": nodeName},
	}, []string{"volume"})

	metrics.Registry.MustRegister(vgTotalBytes)
	metrics.Registry.MustRegister(vgFreeBytes)
	metrics.Registry.MustRegister(volumeTotalBytes)
	metrics.Registry.MustRegister(volumeUsedBytes)

	return &metricsExporter{
		nodeName:         nodeName,
		volume:           volume,
		vgFreeBytes:      vgFreeBytes,
		vgTotalBytes:     vgTotalBytes,
		volumeTotalBytes: volumeTotalBytes,
		volumeUsedBytes:  volumeUsedBytes,
	}
}

// Start implements controller-runtime's manager.Runnable.
func (m *metricsExporter) Start(ctx context.Context) error {
	metricsCh := make(chan DeviceMetrics)
	volumeCh := make(chan VolumeMetrics)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case met := <-metricsCh:
				m.vgTotalBytes.WithLabelValues(met.DeviceGroup).Set(float64(met.TotalBytes))
				m.vgFreeBytes.WithLabelValues(met.DeviceGroup).Set(float64(met.FreeBytes))
			case vc := <-volumeCh:
				m.volumeTotalBytes.WithLabelValues(vc.Volume).Set(float64(vc.TotalBytes))
				m.volumeUsedBytes.WithLabelValues(vc.Volume).Set(vc.UsedBytes)
			}
		}
	}()

	ticker := time.Tick(10 * time.Minute)
	for range ticker {
		vgList, err := m.volume.GetCurrentVgStruct()
		if err == nil && len(vgList) > 0 {
			for _, vg := range vgList {
				metricsCh <- DeviceMetrics{
					FreeBytes:   vg.VGFree,
					TotalBytes:  vg.VGSize,
					DeviceGroup: vg.VGName,
				}
			}
		}

		volumeList, err := m.volume.VolumeList("", "")

		if err == nil && len(volumeList) > 0 {
			for _, v := range volumeList {
				if !strings.HasPrefix(v.LVName, "volume") {
					continue
				}
				volumeCh <- VolumeMetrics{
					Volume:     v.LVName,
					TotalBytes: v.LVSize,
					UsedBytes:  float64(v.LVSize) * v.DataPercent / 100,
				}
			}
		}
	}
	return nil
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}
