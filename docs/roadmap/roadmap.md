## Carina Roadmap

- [Roadmap](#roadmap)
  - [v0.9.1](#v0.9.1)
    - [Allow pod migrating to other nodes when host is notReady](#allow-pod-migrating-to-other-nodes-when-host-is-notReady)
    - [helm install](#helm-install)
    - [Seperate different disks of same type into multiple groups](#seperate-different-disks-of-same-type-into-multiple-groups)
  - [v0.10.0](#v0.10.0)
    - [provisioning raw disk](#provisioning-raw-disk)
    - [velero intergration](#velero-intergration)
  - [v1.0.0](#v1.0.0)
    - [RAID management](#RAID-management)
    - [E2E Testing](#e2e-testing)
  - [v1.1.0](#v1.1.0)
    - [support NVME disks](#support-NVME-disks)
    - [be SMART aware](#be-SMART-aware)
    - [more comprehensive metrics](#more-comprehensive-metrics)
  - [v1.2.0](#v1.2.0)
    - [PVC auto sizing](#PVC-auto-sizing)
    - [scheduling based on near realtime loading](#scheduling-based-on-near-realtime-loading)
    - [cgroup V2](#cgroup-V2)
  - [v1.3.0](#v1.3.0)
    - [E2E checksum](#E2E-checksum)
    - [data encryption](#data-encryption)
    - [descheduling](#descheduling)



## v0.9.1

### Allow pod migrating to other nodes when host is notReady

Currently, when node enters NotReady state and kubernetes-shceduler tries to replace it on other nodes,
the carina scheduler will bind it the the-notready-node again. This works fine if it's just the pod fails
and the newly created pod will reuse the local volume again. But if the node is indeed failed, we should
reschedule the pod to give it another change, although the newly borned pod will have an empty volume.

This will fix [#14](https://github.com/carina-io/carina/issues/14).


### Helm install

Use helm chart for ease of installation、uninstallation、upgrade。


### Separate different disks of same type into multiple groups

Currently, carina groups disks with its type. However, some workloads may prefer using spereated disks
 against others. For now the capacity and allocatable resources will remain the same. 

 This will fix [#10](https://github.com/carina-io/carina/issues/10).

## v0.10.0


### provisioning raw disk

Provides raw disk or partitions to workload, without LVM management. For example, user may request
a raw disk exclusively, or part of disk.

### velero integration

Using velero to backup carina PV to S3.

## v1.0.0

### RAID management

Using RAID to manage disks on baremetal. User can configure RAID level due to needs. When disk
fails, carina can find the failed disk and try to rebuild the RAID if new disk is plugged in.

## v1.1.0

### support NVME disks

support NVME disks

### be SMART aware

Carina should get SMART info for HDD and SSD devices. Issue a warning if found bad sectors or
SSD is dying.

### more comprehensive metrics

Report raw disk and PV's comprehensive metrics, likes IOPS、bandwidth、iotop and so on.

## v1.2.0

### PVC auto sizing

User can use annottion to enable PVC auto sizeing. So if one PV is 80% full, carina will
automatically expanding it without user intervention.

### scheduling based on near realtime loading

Currently carina scheduleing based on node's capacity and allocatable resources. However,
node's load maybe very heavy while it's having lots of free disk spaces. Carina should be
load-aware.

### cgroup V2

Carina should support cgroup V2 for disk throttling to have better experience for buffered IO.

## v1.3.0

### E2E checksum

Ensure read out what exactly been written.

### data encryption

Some workload may prefer safety with performance.

### descheduling

When node's load becomes really heavy, carina can evict some workload with lower priority. The
workload priority is same with kubernetes defines.
