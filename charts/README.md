# Install CSI driver with Helm 3

## Prerequisites

- [install Helm](https://helm.sh/docs/intro/quickstart/#install-helm)
- [add the Chart repository](#add-the-helm-chart-repository)

### add the Helm chart repository

```console
helm repo add carina-csi-driver https://raw.githubusercontent.com/carina-io/charts/main
```

### search for all available chart versions

```console
helm search repo -l carina-csi-driver
```

### update the repository

```console
helm repo update
```

---

## Carina Disk CSI Driver V0.9.0

### install a specific version

```console
helm install carina-csi-driver carina-csi-driver/carina-csi-driver --namespace kube-system --version v0.9.0
```

### uninstall CSI driver

```console
helm uninstall carina-csi-driver -n kube-system
```

### latest chart configuration

#### Parameters

The following table lists the configurable parameters of the latest  Disk CSI Driver chart and default values.

| Parameter                                         | Description                                                                                                | Default                                                      |
| ------------------------------------------------- |------------------------------------------------------------------------------------------------------------| ------------------------------------------------------------ |
| `driver.name`                                     | alternative driver name                                                                                    | `csi.carina.com` |
| `driver.podInfoOnMount`                           | true                                                                                                       | `true` |
| `driver.volumeLifecycleModes`                     | Persistent                                                                                                 | `Persistent` |
| `image.carina.repository`                         | carina-csi-driver docker image                                                                             | `ghcr.io/kilnfi/carina`   |
| `image.carina.tag`                                | carina-csi-driver docker image tag                                                                         | `<tagPrefix>v<appVersion>`  |
| `image.carina.tagPrefix`                          | prefix of the default image tag                                                                            | `controller-`  |
| `image.carina.pullPolicy`                         | carina-csi-driver image pull policy                                                                        | `IfNotPresent`   |
| `image.scheduler.repository`                      | carina-scheduler docker image                                                                              | `ghcr.io/kilnfi/carina`   |
| `image.scheduler.tag`                             | carina-scheduler docker image tag                                                                          | `<tagPrefix>v<appVersion>`  |
| `image.scheduler.tagPrefix`                       | prefix of the default scheduler image tag                                                                  | `scheduler-`  |
| `image.scheduler.pullPolicy`                      | carina-scheduler image pull policy                                                                         | `IfNotPresent`   |
| `image.csiProvisioner.repository`                 | csi-provisioner docker image                                                                               | `registry.k8s.io/sig-storage/csi-provisioner`  |
| `image.csiProvisioner.tag`                        | csi-provisioner docker image tag                                                                           | `v6.4.0`  |
| `image.csiProvisioner.pullPolicy`                 | csi-provisioner image pull policy                                                                          | `IfNotPresent`  |
| `image.csiResizer.repository`                     | csi-resizer docker image                                                                                   | `registry.k8s.io/sig-storage/csi-resizer`    |
| `image.csiResizer.tag`                            | csi-resizer docker image tag                                                                               | `v2.3.0`         |
| `image.csiResizer.pullPolicy`                     | csi-resizer image pull policy                                                                              | `IfNotPresent`        |
| `image.nodeDriverRegistrar.repository`            | csi-node-driver-registrar docker image                                                                     | `registry.k8s.io/sig-storage/csi-node-driver-registrar` |
| `image.nodeDriverRegistrar.tag`                   | csi-node-driver-registrar docker image tag                                                                 | `v2.18.0`                   |
| `image.nodeDriverRegistrar.pullPolicy`            | csi-node-driver-registrar image pull policy                                                                | `IfNotPresent`              |
| `imagePullSecrets`                                | Specify docker-registry secret names as an array                                                           | []         |
| `serviceAccount.create`                           | whether create service account of csi-carina-controller, csi-carina-node                                   | `true`   |                                                |
| `serviceAccount.controller`                       | name of service account for csi-carina-controller                                                          | `carina-csi-controller`       |
| `serviceAccount.node`                             | name of service account for csi-carina-node                                                                | `carina-csi-node`         |
| `serviceAccount.scheduler`                        | name of service account for carina-scheduler                                                               | `carina-scheduler-sa`         |
| `rbac.create`                                     | whether create rbac of csi-carina-controller                                                               | `true`           |
| `rbac.name`                                       | driver name in rbac role                                                                                   | `carina`         |
| `controller.name`                                 | name of driver deployment                                                                                  | `csi-carina-controller` |
| `controller.replicas`                             | the replicas of csi-carina-controller                                                                      | `2`           |
| `controller.metricsPort`                          | metrics port of csi-carina-controller                                                                      | `29604`         |
| `controller.webhookPort`                          | webhookPort port of csi-carina-controller                                                                  | `8443`         |
| `controller.tolerations`                          | controller pod tolerations                                                                                 |     |
| `controller.podLabels`                            | controller pod podLabels                                                                                   |     |
| `controller.hostNetwork`                          | `hostNetwork` setting on controller driver(could be disabled if controller does not depend on MSI setting) | `true`     | `true`, `false`
| `controller.resources.csiProvisioner.limits.cpu`      | csi-provisioner cpu limits                                                                                 | 200m                                                           |
| `controller.resources.csiProvisioner.limits.memory`   | csi-provisioner memory limits                                                                              | 500Mi                                                          |
| `controller.resources.csiProvisioner.requests.cpu`    | csi-provisioner cpu requests limits                                                                        | 10m                                                            |
| `controller.resources.csiProvisioner.requests.memory` | csi-provisioner memory requests limits                                                                     | 20Mi                                                           |
| `controller.resources.csiResizer.limits.cpu`          | csi-resizer cpu limits                                                                                     | 200m                                                           |
| `controller.resources.csiResizer.limits.memory`       | csi-resizer memory limits                                                                                  | 500Mi                                                          |
| `controller.resources.csiResizer.requests.cpu`        | csi-resizer cpu requests limits                                                                            | 10m                                                            |
| `controller.resources.csiResizer.requests.memory`     | csi-resizer memory requests limits                                                                         | 20Mi                                                           |
| `controller.resources.carina.limits.cpu`           | carina cpu limits                                                                                          | 300m                                                           |
| `controller.resources.carina.limits.memory`        | carina memory limits                                                                                       | 500Mi                                                          |
| `controller.resources.carina.requests.cpu`         | carina cpu requests limits                                                                                 | 10m                                                            |
| `controller.resources.carina.requests.memory`      | carina memory requests limits                                                                              | 20Mi    |
| `scheduler.name`                                  | name of scheduler deployment                                                                               | `carina-scheduler` |
| `scheduler.replicas`                              | the replicas of carina-scheduler                                                                           | `1` |
| `scheduler.resources`                             | carina-scheduler resources                                                                                 | 100m / 128Mi |
| `scheduler.nodeSelector`, `scheduler.tolerations`, `scheduler.affinity` | scheduler pod placement                                              |     |
| `node.name`                                       | name of driver daemonset                                                                                   |`csi-carina-node`       |
| `node.maxUnavailable`                             | `maxUnavailable` value of driver node daemonset                                                            | `1`
| `node.metricsPort`                                | metrics port of csi-carina-node                                                                            |`29091`         |
| `node.httpPort`                                   | httpPort port of csi-carina-node                                                                           |`29090`           |
| `node.kubelet`                                   | configure kubelet directory path on  agent node                                                            | `/var/lib/kubelet`        |
| `node.initContainer.modprobe`                    | configure lib module(available values: `dm_snapshot`, `dm_mirror`,`dm_thin_pool`,`bcache`)                 | `dm_snapshot`, `dm_mirror`,`dm_thin_pool`   |
| `node.tolerations`                               | node driver tolerations                                                                                    |      |
| `node.podLabels`                                 | node pod podLabels                                                                                         |        |
| `node.hostNetwork`                               | `hostNetwork` setting on  node driver(could be disabled if perfProfile is `none`)                          | `true`           | `true`, `false`
| `node.nodeAffinity`                                 | node pod nodeAffinity                                                                                      |                                                              |
| `node.resources.nodeDriverRegistrar.limits.cpu`       | csi-node-driver-registrar cpu limits                                                                       | 200m                                                           |
| `node.resources.nodeDriverRegistrar.limits.memory`    | csi-node-driver-registrar memory limits                                                                    | 100Mi                                                          |
| `node.resources.nodeDriverRegistrar.requests.cpu`     | csi-node-driver-registrar cpu requests limits                                                              | 10m                                                            |
| `node.resources.nodeDriverRegistrar.requests.memory`  | csi-node-driver-registrar memory requests limits                                                           | 20Mi                                                           |
| `node.resources.carina.limits.cpu`                 | carina cpu limits                                                                                          | 200m         |
| `node.resources.carina.limits.memory`              | carina memory limits                                                                                       | 200Mi           |
| `node.resources.carina.requests.cpu`               | carina cpu requests limits                                                                                 | 10m            |
| `node.resources.carina.requests.memory`            | carina memory requests limits                                                                              | 20Mi         |
| `node.logDir`                                      | node pod logDir                                                                                            |/var/log/carina/  |
| `node.configDir`                                   | node pod configDir                                                                                         |/etc/carina      |
| `installCRDs`                                      | install crd                                                                                                |true  |      |
| `serviceMonitor.enable`                            | controller minitor serviceMonitor                                                                          |true  |      |
| `webhook.enable`                                   | controller webhook                                                                                         |true  |      |
---

#### storageClass

```console
 kubectl --namespace={{ .Release.Namespace }} get sc 
```
