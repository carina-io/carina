#### topologyKey

When starts up, carina-node will label each node with `topology.carina.storage.io/node=${nodename}`. For storageclass, user can set `allowedTopologies` to affect pod scheduling.

Creating storageclass with `kubectl apply -f storageclass.yaml`

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
volumeBindingMode: Immediate
mountOptions:
allowedTopologies:
  - matchLabelExpressions:
      - key: beta.kubernetes.io/os
        values:
          - linux
      - key: kubernetes.io/hostname
        values:
          - 10.20.9.153
          - 10.20.9.154
```

- `allowedTopologies` policy only works with `volumeBindingMode: Immediate`.  


Example as follows:

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