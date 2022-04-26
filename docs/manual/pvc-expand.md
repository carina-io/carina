#### expanding PVC

Carina support expanding PVC online, so user can resize carina pvc as needed.

```shell
$ kubectl get pvc -n carina
NAMESPACE  NAME        STATUS  VOLUME                                    Capacity  STORAGECLASS  AGE
carina     carina-pvc  Bound   pvc-80ede42a-90c3-4488-b3ca-85dbb8cd6c22  7G        carina-sc     20d
```

Expanding it online.

```shell
$ kubectl patch pvc/carina-pvc \
  --namespace "carina" \
  --patch '{"spec": {"resources": {"requests": {"storage": "15Gi"}}}}'
```

Check if expanding works.

```shell
$ kubectl exec -it web-server -n carina bash
$ df -h
Filesystem                                 Size  Used Avail Use% Mounted on
overlay                                    199G   17G  183G   9% /
tmpfs                                      64M     0   64M   0% /dev
/dev/vda2                                  199G   17G  183G   9% /conf
/dev/carina-vg-hdd/volume....              15G     0   64M   0% /www/nginx/work
tmpfs                                      3.9G     0  3.9G   0% /tmp/k8s-webhook-server/serving-certs
```

Note, if using cache tiering PVC, then user need to restart the pod to make the expanding work. 