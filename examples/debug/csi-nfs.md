docker run -d --privileged --restart=always \
-v /var/lib/kubelet:/nfs \
-e NFS_EXPORT_DIR_1=/nfs \
-e NFS_EXPORT_DOMAIN_1=\* \
-e NFS_EXPORT_OPTIONS_1=ro,insecure,no_subtree_check,no_root_squash,fsid=1 \
-p 111:111 -p 111:111/udp \
-p 2049:2049 -p 2049:2049/udp \
-p 32765:32765 -p 32765:32765/udp \
-p 32766:32766 -p 32766:32766/udp \
-p 32767:32767 -p 32767:32767/udp \
fuzzle/docker-nfs-server:latest

busybox mount -v -t nfs -o remount,rw,nfsvers=3,nolock,proto=udp,port=2049 10.20.9.154:/nfs /var/lib/kubelet