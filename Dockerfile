# Build the manager binary
FROM golang:1.19.0-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/github.com/carina-io/carina
ENV GOMODCACHE=$WORKSPACE/vendor

WORKDIR $WORKSPACE
ADD . .

# Build
RUN echo Commit: `git log --pretty='%s%b%B' -n 1`
RUN cd $WORKSPACE/cmd/carina-node && go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o /tmp/carina-node .
RUN cd $WORKSPACE/cmd/carina-controller && go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o /tmp/carina-controller .

FROM registry.cn-hangzhou.aliyuncs.com/carina/centos-lvm2:runtime-20220108

# copy binary file
COPY --from=builder /tmp/carina-node /usr/bin/
COPY --from=builder /tmp/carina-controller /usr/bin/
COPY --from=builder /workspace/github.com/carina-io/carina/debug/hack/config.json /etc/carina/
RUN chmod +x /usr/bin/carina-node && chmod +x /usr/bin/carina-controller

# Update time zone to Asia-Shanghai
COPY --from=builder /workspace/github.com/carina-io/carina/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' > /etc/timezone

CMD ["echo carina-node carina-controller"]
