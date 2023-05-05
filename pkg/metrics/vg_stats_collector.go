package metrics

import (
	"errors"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	vgSubSystem string = "volume_group_stats"
)

var (
	vgStatLabels     = []string{"device_group"}
	vgTotalBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, vgSubSystem, "capacity_bytes_total"),
		"The number of lvm vg total bytes.",
		vgStatLabels,
		constLabels,
	)
	vgUsedBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, vgSubSystem, "capacity_bytes_used"),
		"The number of lvm vg used bytes.",
		vgStatLabels,
		constLabels,
	)
	lvTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, vgSubSystem, "lv_total"),
		"The number of lv total.",
		vgStatLabels,
		constLabels,
	)
	pvTotalDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, vgSubSystem, "pv_total"),
		"The number of pv total.",
		vgStatLabels,
		constLabels,
	)
)

type vgStatsCollector struct {
	descs []typedFactorDesc
	dm    *deviceManager.DeviceManager
}

func newVolumeGroupStatsCollector(dm *deviceManager.DeviceManager) (Collector, error) {
	return &vgStatsCollector{
		descs: []typedFactorDesc{
			{desc: vgTotalBytesDesc, valueType: prometheus.GaugeValue},
			{desc: vgUsedBytesDesc, valueType: prometheus.GaugeValue},
			{desc: lvTotalDesc, valueType: prometheus.GaugeValue},
			{desc: pvTotalDesc, valueType: prometheus.GaugeValue},
		},
		dm: dm,
	}, nil
}

func (v *vgStatsCollector) Name() string {
	return "vg_stats"
}

func (v *vgStatsCollector) Update(ch chan<- prometheus.Metric) error {
	diskSelectGroup := v.dm.GetNodeDiskSelectGroup()
	vgList, err := v.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		return errors.New("couldn't get volume group:" + err.Error())
	}
	for _, vg := range vgList {
		if _, ok := diskSelectGroup[vg.VGName]; !ok {
			continue
		}
		// need keep order with desc
		for i, val := range []float64{
			float64(vg.VGSize),
			float64(vg.VGSize - vg.VGFree),
			float64(vg.LVCount),
			float64(vg.PVCount),
		} {
			if i >= len(v.descs) {
				break
			}
			ch <- v.descs[i].mustNewConstMetric(val, vg.VGName)
		}

	}
	return nil
}
