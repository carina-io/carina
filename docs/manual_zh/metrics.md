指标监控

- carina-node 为host网络模式部署并监听 `8080`端口，其中8080为metrics，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
  ```

  备注：若是修改监听端口，务必同步修改`service：csi-carina-node`

- carina-controller 监听`8080 8443`，其中8080为metrics、8443为webhook，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
  ```

- carina 指标

| 指标                                           | 描述                   |
| ---------------------------------------------- | ---------------------- |
| carina_scrape_collector_duration_seconds       | 收集器持续时间         |
| carina_scrape_collector_success                | 收集器成功次数         |
| carina_volume_group_stats_capacity_bytes_total | vg卷组容量             |
| carina_volume_group_stats_capacity_bytes_used  | vg卷组使用量           |
| carina_volume_group_stats_lv_total             | 节点lv数量             |
| carina_volume_group_stats_pv_total             | 节点pv数量             |
| carina_volume_stats_reads_completed_total      | 成功读取的总数         |
| carina_volume_stats_reads_merged_total         | 合并的读的总数         |
| carina_volume_stats_read_bytes_total           | 成功读取的字节总数     |
| carina_volume_stats_read_time_seconds_total    | 所有读花费的总秒数     |
| carina_volume_stats_writes_completed_total     | 成功完成写的总数       |
| carina_volume_stats_writes_merged_total        | 合并写的数量           |
| carina_volume_stats_write_bytes_total          | 成功写入的总字节数     |
| carina_volume_stats_write_time_seconds_total   | 所有写操作花费的总秒数 |
| carina_volume_stats_io_now                     | 当前正在处理的I/O秒数  |
| carina_volume_stats_io_time_seconds_total      | I/O花费的总秒数        |

- carina 提供了丰富的存储卷指标，kubelet本身也暴露的 PVC 容量等指标，在 Grafana Kubernetes 内置视图，可以看到此模板。注意具体 PVC 存储容量指标只有当该 PVC 被使用并且挂载到该节点时才会显示


