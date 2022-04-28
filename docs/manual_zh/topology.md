#### 本地拓扑卷

拓扑卷的目的主要是在调度pvc时，将pv调度到某些特定节点，carina支持topo卷调度，介绍如下

在carina启动时会为每个节点打上`topology.carina.storage.io/node=nodename`标签，当在storageclass设置topology参数时将按照要求选择存储卷

创建topology存储类 `kubectl apply -f storageclass.yaml`

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: csi-carina-sc
provisioner: carina.storage.io
parameters:
  # file system
  csi.storage.k8s.io/fstype: xfs
  # disk group
  carina.storage.io/disk-group-name: hdd
reclaimPolicy: Delete
allowVolumeExpansion: true
# 创建pvc后立即创建pv,WaitForFirstConsumer表示被容器绑定调度后再创建pv
volumeBindingMode: Immediate
mountOptions:
allowedTopologies:
  - matchLabelExpressions:
      - key: beta.kubernetes.io/os
        values:
          - linux
          - amd64
      - key: kubernetes.io/hostname
        values:
          - 10.20.9.153
          - 10.20.9.154
```

- 注意只有`volumeBindingMode: Immediate`类型的才支持根据`allowedTopologies`选择pv所在节点，当`volumeBindingMode: WaitForFirstConsumer`时，pv选择节点将根据pod的topology设置进行调度

创建一个包含topology选择的statefulset，如下`kubectl apply -f statefulset.yaml`，`topologyKey`表示该POD只会调度到存在carina存储驱动的节点上

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: carina-topo-stateful
  namespace: carina
spec:
  serviceName: "nginx"
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: kubernetes.io/os
                    operator: In
                    values:
                      - linux
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - nginx
              topologyKey: topology.carina.storage.io/node
      containers:
        - name: nginx
          image: nginx
          imagePullPolicy: "IfNotPresent"
          ports:
            - containerPort: 80
              name: web
          volumeMounts:
            - name: www
              mountPath: /usr/share/nginx/html
            - name: logs
              mountPath: /logs
  volumeClaimTemplates:
    - metadata:
        name: www
      spec:
        accessModes: [ "ReadWriteOnce" ]
        storageClassName: csi-carina-sc
        resources:
          requests:
            storage: 10Gi
    - metadata:
        name: logs
      spec:
        accessModes: [ "ReadWriteOnce" ]
        storageClassName: csi-carina-sc
        resources:
          requests:
            storage: 5Gi
```

