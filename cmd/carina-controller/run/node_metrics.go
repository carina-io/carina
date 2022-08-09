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

package run

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/carina-io/carina/api"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/log"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

const metricsNamespace = "carina"

// DeviceMetrics Device Metrics
type DeviceMetrics struct {
	nodeName    string
	FreeBytes   uint64
	TotalBytes  uint64
	DeviceGroup string
}

// VolumeMetrics volume Metrics
type VolumeMetrics struct {
	nodeName   string
	Volume     string
	TotalBytes uint64
	UsedBytes  float64
}

type metricsExporter struct {
	vgFreeBytes      *prometheus.GaugeVec
	vgTotalBytes     *prometheus.GaugeVec
	volumeTotalBytes *prometheus.GaugeVec
	volumeUsedBytes  *prometheus.GaugeVec
}

var _ manager.LeaderElectionRunnable = &metricsExporter{}

// NewMetricsExporter creates controller-runtime's manager.Runnable to run
// a metrics exporter for a node.
func newMetricsExporter() manager.Runnable {
	vgFreeBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "devicegroup",
		Name:        "vg_free_bytes",
		Help:        "LVM VG free bytes",
		ConstLabels: prometheus.Labels{},
	}, []string{"node", "device_group"})

	vgTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "devicegroup",
		Name:        "vg_total_bytes",
		Help:        "LVM VG total bytes",
		ConstLabels: prometheus.Labels{},
	}, []string{"node", "device_group"})

	volumeTotalBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volume",
		Name:        "volume_total_bytes",
		Help:        "LVM Volume total bytes",
		ConstLabels: prometheus.Labels{},
	}, []string{"node", "volume"})

	volumeUsedBytes := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   metricsNamespace,
		Subsystem:   "volume",
		Name:        "volume_used_bytes",
		Help:        "LVM volume used bytes",
		ConstLabels: prometheus.Labels{},
	}, []string{"node", "volume"})

	metrics.Registry.MustRegister(vgTotalBytes)
	metrics.Registry.MustRegister(vgFreeBytes)
	metrics.Registry.MustRegister(volumeTotalBytes)
	metrics.Registry.MustRegister(volumeUsedBytes)

	return &metricsExporter{
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
				m.vgTotalBytes.WithLabelValues(met.nodeName, met.DeviceGroup).Set(float64(met.TotalBytes))
				m.vgFreeBytes.WithLabelValues(met.nodeName, met.DeviceGroup).Set(float64(met.FreeBytes))
			case vc := <-volumeCh:
				m.volumeTotalBytes.WithLabelValues(vc.nodeName, vc.Volume).Set(float64(vc.TotalBytes))
				m.volumeUsedBytes.WithLabelValues(vc.nodeName, vc.Volume).Set(vc.UsedBytes)
			}
		}
	}()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			dm, err := vgMetrics()
			if err == nil && len(dm) > 0 {
				for _, m := range dm {
					metricsCh <- m
				}
			}
			vm, err := volumeMetrics()
			if err == nil && len(vm) > 0 {
				for _, v := range vm {
					volumeCh <- v
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (m *metricsExporter) NeedLeaderElection() bool {
	return false
}

func vgMetrics() ([]DeviceMetrics, error) {
	metricsResult := []DeviceMetrics{}
	endpoints, err := getEndpoints()
	if err != nil {
		return metricsResult, err
	}
	result := map[string][]api.VgGroup{}

	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/devicegroup", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		r := []api.VgGroup{}
		err = json.Unmarshal(body, &r)
		if err != nil {
			return metricsResult, err
		}
		result[ep.NodeName] = r
	}
	for nodeName, vg := range result {
		for _, v := range vg {
			metricsResult = append(metricsResult, DeviceMetrics{
				nodeName:    nodeName,
				FreeBytes:   v.VGFree,
				TotalBytes:  v.VGSize,
				DeviceGroup: v.VGName,
			})
		}
	}

	return metricsResult, nil
}

func volumeMetrics() ([]VolumeMetrics, error) {
	metricsResult := []VolumeMetrics{}
	endpoints, err := getEndpoints()
	if err != nil {
		return metricsResult, err
	}
	result := map[string][]types.LvInfo{}
	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/volume", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		r := []types.LvInfo{}
		err = json.Unmarshal(body, &r)
		if err != nil {
			return metricsResult, err
		}
		result[ep.NodeName] = r
	}
	for nodeName, lv := range result {
		for _, v := range lv {
			if !strings.HasPrefix(v.LVName, "volume") {
				continue
			}
			metricsResult = append(metricsResult, VolumeMetrics{
				nodeName:   nodeName,
				Volume:     v.LVName,
				TotalBytes: v.LVSize,
				UsedBytes:  float64(v.LVSize) * v.DataPercent / 100,
			})
		}
	}

	return metricsResult, nil
}
