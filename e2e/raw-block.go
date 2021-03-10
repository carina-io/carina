package e2e

var rawPvc = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: raw-block-pvc
  namespace: carina
spec:
  accessModes:
    - ReadWriteOnce
  volumeMode: Block
  resources:
    requests:
      storage: 13Gi
  storageClassName: csi-carina-sc1
`

var rawPod = `
apiVersion: v1
kind: Pod
metadata:
  name: carina-block-pod
  namespace: carina
spec:
  containers:
    - name: centos
      securityContext:
        capabilities:
          add: ["SYS_RAWIO"]
      image: centos:latest
      command: ["/bin/sleep", "infinity"]
      volumeDevices:
        - name: data
          devicePath: /dev/xvda
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: raw-block-pvc
`
