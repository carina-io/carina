## 示例来源

This directory contains sample YAML config files that can be used for exploring Velero.

* `minio/`: Used in the [Quickstart][0] to set up [Minio][1], a local S3-compatible object storage service. It provides a convenient way to test Velero without tying you to a specific cloud provider.

* `nginx-app/`: A sample nginx app that can be used to test backups and restores.


[0]: https://velero.io/docs/main/contributions/minio/
[1]: https://github.com/minio/minio


## 1 安装velero客户端
```
wget https://mirror.ghproxy.com/https://github.com/vmware-tanzu/velero/releases/download/v1.6.3/velero-v1.6.3-darwin-amd64.tar.gz 
or 
wget https://download.fastgit.org/vmware-tanzu/velero/releases/download/v1.6.3/velero-v1.6.3-darwin-amd64.tar.gz
tar -zxvf velero-v1.6.3-darwin-amd64.tar.gz && cd velero-v1.6.3-darwin-amd64 
mv velero /usr/local/bin && chmod +x /usr/local/bin/velero 
velero version
```

## 2 安装minio 
- 编辑00-minio-deployment.yaml 
cat > 00-minio-deployment.yaml  << EOF 
```
apiVersion: v1 
kind: Namespace 
metadata: 
  name: velero 
--- 
apiVersion: apps/v1 
kind: Deployment 
metadata: 
  namespace: velero 
  name: minio 
  labels: 
    component: minio 
spec: 
  strategy: 
    type: Recreate 
  selector: 
    matchLabels: 
      component: minio 
  template: 
    metadata: 
      labels: 
        component: minio 
    spec: 
      volumes: 
      - name: storage 
        emptyDir: {} 
      - name: config 
        emptyDir: {} 
      containers: 
      - name: minio 
        image: minio/minio:latest 
        imagePullPolicy: IfNotPresent 
        args: 
        - server 
        - /storage 
        - --config-dir=/config 
        - --console-address=:9001 
        env: 
        - name: MINIO_ACCESS_KEY 
          value: "minio" 
        - name: MINIO_SECRET_KEY 
          value: "minio123" 
        ports: 
        - containerPort: 9000 
        - containerPort: 9001 
        volumeMounts: 
        - name: storage 
          mountPath: "/storage" 
        - name: config 
          mountPath: "/config" 
--- 
apiVersion: v1 
kind: Service 
metadata: 
  namespace: velero 
  name: minio 
  labels: 
    component: minio 
spec: 
  type: NodePort 
  ports: 
    - name: api 
      port: 9000 
      targetPort: 9000 
    - name: console 
      port: 9001 
      targetPort: 9001 
  selector: 
    component: minio 
--- 
apiVersion: batch/v1 
kind: Job 
metadata: 
  namespace: velero 
  name: minio-setup 
  labels: 
    component: minio 
spec: 
  template: 
    metadata: 
      name: minio-setup 
    spec: 
      restartPolicy: OnFailure 
      volumes: 
      - name: config 
        emptyDir: {} 
      containers: 
      - name: mc 
        image: minio/mc:latest 
        imagePullPolicy: IfNotPresent 
        command: 
        - /bin/sh 
        - -c 
        - "mc --config-dir=/config config host add velero http://minio:9000 minio minio123 && mc --config-dir=/config mb -p velero/velero" 
        volumeMounts: 
        - name: config 
          mountPath: "/config" 
```
```
kubectl apply -f ./00-minio-deployment.yaml 
ubuntu@LAPTOP-4FT6HT3J:~/www/src/github.com/carina-io/velero$ kubectl get pods -n velero 
NAME                     READY   STATUS              RESTARTS   AGE
minio-58dc5cf789-z2777   0/1     ContainerCreating   0          14s
minio-setup-dz4jb        0/1     ContainerCreating   0          6s
ubuntu@LAPTOP-4FT6HT3J:~/www/src/github.com/carina-io/velero$ kubectl get svc  -n velero 
NAME    TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)                         AGE
minio   NodePort   10.96.13.35   <none>        9000:30693/TCP,9001:32351/TCP   17s
```
- 创建minio凭证 
```
vi credentials-velero
```
```
i
```

## 安装velero服务端 ，使用s3 作为存储

- 使用官方提供的restic组件备份pv
```
velero install    \
  --image velero/velero:v1.6.3  \
	 --plugins velero/velero-plugin-for-aws:v1.0.0  \
	 --provider aws   \
	 --bucket velero   \
	 --namespace velero  \
	 --secret-file ./credentials-velero   \
	 --velero-pod-cpu-request 200m   \
	 --velero-pod-mem-request 200Mi   \
	 --velero-pod-cpu-limit 1000m  \
	 --velero-pod-mem-limit 1000Mi   \
	 --use-volume-snapshots=false   \
	 --use-restic   \
	 --restic-pod-cpu-request 200m   \
	 --restic-pod-mem-request 200Mi   \
	 --restic-pod-cpu-limit 1000m  \
	 --restic-pod-mem-limit 1000Mi  \
	 --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000
```

-  参数说明
```
参数说明：
--provider：声明使用的 Velero 插件类型。
--plugins：使用 S3 API 兼容插件 “velero-plugin-for-aws ”。
--bucket：在腾讯云 COS 创建的存储桶名。
--secret-file：访问 COS 的访问凭证文件，见上面创建的 “credentials-velero”凭证文件。
--use-restic：使用开源免费备份工具 restic 备份和还原持久卷数据。
--default-volumes-to-restic：使用 restic 来备份所有Pod卷，前提是需要开启 --use-restic 参数。
--backup-location-config：备份存储桶访问相关配置。
--region：兼容 S3 API 的 COS 存储桶地区，例如创建地区是广州的话，region 参数值为“ap-guangzhou”。
--s3ForcePathStyle：使用 S3 文件路径格式。
--s3Url：COS 兼容的 S3 API 访问地址
--use-volume-snapshots=false 来关闭存储卷数据快照备份。
```
-  安装命令执行完成后，等待 Velero 和 restic 工作负载就绪后，查看配置的存储位置是否可用。
```
 velero backup-location get 
```


#### 使用 Minio

```
apiVersion: velero.io/v1	
kind: BackupStorageLocation	
metadata:	
  name: default	
  namespace: velero	
spec:	
# 只有 aws gcp azure	
  provider: aws	
  objectStorage:	
    bucket: myBucket	
    prefix: backup	
  config:	
    region: us-west-2		
    profile: "default"	
    s3ForcePathStyle: "false"	
    s3Url: http://minio:9000

```
#### 我们将使用 Restic 来对 PV 进行备份，不过现阶段通过 Restic 备份会有一些限制。

- 不支持备份 hostPath
- 备份数据标志只能通过 Pod 来识别
- 单线程操作大量文件比较慢

#### 使用 Velero/Restic 进行数据备份和恢复 必须先给 Pod 加注解
- kubectl -n YOUR_POD_NAMESPACE annotate pod/YOUR_POD_NAME backup.velero.io/backup-volumes=YOUR_VOLUME_NAME_1,YOUR_VOLUME_NAME_2,...
- 例如

```
 kubectl apply -f examples/nginx-app/with-pv.yaml
 kubectl -n nginx-example annotate pod nginx-deployment-66689547d-z9tbx   backup.velero.io/backup-volumes=nginx-logs
 kubectl get pod -n nginx-example nginx-deployment-66689547d-z9tbx -o jsonpath='{.metadata.annotations}'
```

#### 备份流程
- Velero 客户端调用 Kubernetes APIServer 创建 Backup 这个 CRD 对象
- Backup 控制器 watch 到新的 Backup 对象被创建并执行验证
- Backup 控制器开始执行备份，通过查询 APIServer 来获取资源收集数据进行备份
- Backup 控制器调用对象存储服务，比如 S3 上传备份文件


#### 创建备份数据
>可以所有对象，也可以按类型，名称空间和/或标签过滤对象
- velero create backup NAME [flags]
- velero backup create pvc-backup-1  --snapshot-volumes --include-namespaces nginx-example --default-volumes-to-restic --volume-snapshot-locations default


#### 查看备份任务。
> 当备份任务状态是“Completed”时，错误数为 0 ，说明备份任务完成且没发生任何错误，
```
 velero backup get 
```
> 先临时将备份存储位置更新为只读模式（这可以防止在还原过程中在备份存储位置中创建或删除备份对象）

```
kubectl patch backupstoragelocation default --namespace velero \
    --type merge \
    --patch '{"spec":{"accessMode":"ReadOnly"}}'
```
```
velero backup-location get
NAME      PROVIDER   BUCKET/PREFIX   PHASE     LAST VALIDATED   ACCESS MODE   DEFAULT
default   aws        velero          Unknown   Unknown          ReadWrite     true
```

#### 恢复备份数据

- velero restore create --from-backup <backup-name>
- velero  restore create --from-backup pvc-backup-1 --restore-volumes

#### 查看恢复任务。
```
 velero restore get 
```
> 还原完成后，不要忘记把备份存储位置恢复为读写模式，以便下次备份任务成功使用： 
```
kubectl patch backupstoragelocation default --namespace velero \
   --type merge \
   --patch '{"spec":{"accessMode":"ReadWrite"}}'
```  






