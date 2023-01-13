#### Metrics

- carina-node runs in hostNetwork mode and listens to `8080`.

  ```shell
          - "--metrics-addr=:8080"
  ```

If changing those ports, please also change the csi-carina-node service.

- carina-controller listens to `8080 8443`.

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
  ```

- carina metrics

| Metrics                                        | Description                                             |
| ---------------------------------------------- | ------------------------------------------------------- |
| carina_scrape_collector_duration_seconds       | carina_csi_exporter: Duration of a collector scrape     |
| carina_scrape_collector_success                | carina_csi_exporter: Whether a collector succeeded      |
| carina_volume_group_stats_capacity_bytes_total | The number of lvm vg total bytes                        |
| carina_volume_group_stats_capacity_bytes_used  | The number of lvm vg used bytes                         |
| carina_volume_group_stats_lv_total             | The number of lv total                                  |
| carina_volume_group_stats_pv_total             | The number of pv total                                  |
| carina_volume_stats_reads_completed_total      | The total number of reads completed successfully        |
| carina_volume_stats_reads_merged_total         | The total number of reads merged                        |
| carina_volume_stats_read_bytes_total           | The total number of bytes read successfully             |
| carina_volume_stats_read_time_seconds_total    | The total number of seconds spent by all reads          |
| carina_volume_stats_writes_completed_total     | The total number of writes completed successfully       |
| carina_volume_stats_writes_merged_total        | The number of writes merged                             |
| carina_volume_stats_write_bytes_total          | The total number of bytes write successfully            |
| carina_volume_stats_write_time_seconds_total   | This is the total number of seconds spent by all writes |
| carina_volume_stats_io_now                     | The number of I/Os currently in progress                |
| carina_volume_stats_io_time_seconds_total      | Total seconds spent doing I/Os                          |

- carina provides a wealth of storage volume metrics, and kubelet itself also exposes PVC capacity and other metrics, as seen in the Grafana Kubernetes built-in view of this template. Notice The storage capacity indicator of the PVC is displayed only when the PVC is in use and mounted to the node



