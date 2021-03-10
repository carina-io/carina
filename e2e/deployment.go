package e2e

var deployment1 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment
  namespace: carina
  labels:
    app: web-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server
  template:
    metadata:
      labels:
        app: web-server
    spec:
      containers:
        - name: web-server
          image: docker.io/library/nginx:latest
          volumeMounts:
            - name: mypvc
              mountPath: /var/lib/www/html
      volumes:
        - name: mypvc
          persistentVolumeClaim:
            claimName: csi-carina-pvc1
            readOnly: false
`

var deployment3 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: carina-deployment
  namespace: carina
  labels:
    app: web-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-server
  template:
    metadata:
      labels:
        app: web-server
    spec:
      containers:
        - name: web-server
          image: docker.io/library/nginx:latest
          volumeMounts:
            - name: mypvc
              mountPath: /var/lib/www/html
      volumes:
        - name: mypvc
          persistentVolumeClaim:
            claimName: csi-carina-pvc3
            readOnly: false
`
