#### Metrics

- carina-node runs in hostNetwork mode and listens to `8080 8089`.

  ```shell
          - "--metrics-addr=:8080"
          - "--http-addr=:8089"
  ```

If changing those ports, please also change the csi-carina-node service.

- carina-controller listens to `8080 8443 8089`.

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
          - "--http-addr=:8089"
  ```

- metrics from carina-nodecarina-controller

  ```shell
  	# Free bytes in VG:  carina-devicegroup-vg_free_bytes
  	# Total bytes of VG:  carina-devicegroup-vg_total_bytes
  	# Total bytes of volume:  carina-volume-volume_total_bytes
  	# Used bytes of volume:  carina-volume-volume_used_bytes
  ```

* Volume usage is caculated from LVM, it may diffs with `df -h` about dozens of MB. 
* Carina-controller has all data from each carina-node. So actually, just getting metrics from carina-controller is enough.
* User can deploy serviceMonitor(deployment/kubernetes/prometheus.yaml.tmpl) in case of prometheus. 
* For pvc metrics, user can still query from kubelet.