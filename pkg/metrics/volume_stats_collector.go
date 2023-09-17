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
	"context"
	"errors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/procfs/blockdevice"

	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
)

const (
	volumeSubSystem string = "volume_stats"
	secondsPerTick         = 1.0 / 1000.0
	// Read sectors and write sectors are the "standard UNIX 512-byte sectors, not any device- or filesystem-specific block size."
	// See also https://www.kernel.org/doc/Documentation/block/stat.txt
	unixSectorSize = 512.0
	// need mount proc/sys when container deploy carina node
	procPath = "/host/proc"
)

var (
	deviceStatLabels = []string{"namespace", "pvc", "pv", "device_group"}

	readsCompletedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "reads_completed_total"),
		"The total number of reads completed successfully.",
		deviceStatLabels,
		constLabels,
	)
	readsMergeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "reads_merged_total"),
		"The total number of reads merged.",
		deviceStatLabels,
		constLabels,
	)
	readBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "read_bytes_total"),
		"The total number of bytes read successfully.",
		deviceStatLabels,
		constLabels,
	)
	readTimeMilliSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "read_time_seconds_total"),
		"The total number of seconds spent by all reads.",
		deviceStatLabels,
		constLabels,
	)
	writesCompletedDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "writes_completed_total"),
		"The total number of writes completed successfully.",
		deviceStatLabels,
		constLabels,
	)
	writesMergeDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "writes_merged_total"),
		"The number of writes merged.",
		deviceStatLabels,
		constLabels,
	)
	writeBytesDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "write_bytes_total"),
		"The total number of bytes write successfully.",
		deviceStatLabels,
		constLabels,
	)
	writeTimeMilliSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "write_time_seconds_total"),
		"This is the total number of seconds spent by all writes.",
		deviceStatLabels,
		constLabels,
	)
	iONowDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "io_now"),
		"The number of I/Os currently in progress.",
		deviceStatLabels,
		constLabels,
	)
	iOTimeSecondsDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, volumeSubSystem, "io_time_seconds_total"),
		"Total seconds spent doing I/Os.",
		deviceStatLabels,
		constLabels,
	)
)

type volumeStatsCollector struct {
	descs     []typedFactorDesc
	lvService *k8s.LogicVolumeService
	fs        blockdevice.FS
}

func newVolumeStatsCollector(lvService *k8s.LogicVolumeService) (Collector, error) {
	fs, err := blockdevice.NewFS(procPath, "")
	if err != nil {
		return nil, errors.New("failed to open sysfs:" + err.Error())
	}

	return &volumeStatsCollector{
		descs: []typedFactorDesc{
			{desc: readsCompletedDesc, valueType: prometheus.CounterValue},
			{desc: readsMergeDesc, valueType: prometheus.CounterValue},
			{desc: readBytesDesc, valueType: prometheus.CounterValue},
			{desc: readTimeMilliSecondsDesc, valueType: prometheus.CounterValue},
			{desc: writesCompletedDesc, valueType: prometheus.CounterValue},
			{desc: writesMergeDesc, valueType: prometheus.CounterValue},
			{desc: writeBytesDesc, valueType: prometheus.CounterValue},
			{desc: writeTimeMilliSecondsDesc, valueType: prometheus.CounterValue},
			{desc: iONowDesc, valueType: prometheus.GaugeValue},
			{desc: iOTimeSecondsDesc, valueType: prometheus.CounterValue},
		},
		lvService: lvService,
		fs:        fs,
	}, nil
}

func (v *volumeStatsCollector) Name() string {
	return "volume_stats"
}

func (v *volumeStatsCollector) Update(ch chan<- prometheus.Metric) error {
	diskStats, err := v.fs.ProcDiskstats()
	if err != nil {
		return errors.New("couldn't get diskstats:" + err.Error())
	}
	logicVolumes, err := v.lvService.GetLogicVolumesByNodeName(context.Background(), nodeName, false)
	if err != nil {
		return err
	}
	// TODO shared raw devices need special processing
	for _, logicVolume := range logicVolumes {
		for _, stats := range diskStats {
			if stats.MajorNumber != logicVolume.Status.DeviceMajor || stats.MinorNumber != logicVolume.Status.DeviceMinor {
				continue
			}
			for i, val := range []float64{
				// need keep order with desc
				float64(stats.ReadIOs),
				float64(stats.ReadMerges),
				float64(stats.ReadSectors) * unixSectorSize,
				float64(stats.ReadTicks) * secondsPerTick,
				float64(stats.WriteIOs),
				float64(stats.WriteMerges),
				float64(stats.WriteSectors) * unixSectorSize,
				float64(stats.WriteTicks) * secondsPerTick,
				float64(stats.IOsInProgress),
				float64(stats.IOsTotalTicks) * secondsPerTick,
			} {
				if i >= len(v.descs) {
					break
				}
				ch <- v.descs[i].mustNewConstMetric(val, logicVolume.Spec.NameSpace, logicVolume.Spec.Pvc, logicVolume.Name, logicVolume.Spec.DeviceGroup)
			}
		}
	}
	return nil
}
