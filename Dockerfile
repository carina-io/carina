# Build the manager binary
FROM golang:1.15-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/carina

WORKDIR $WORKSPACE
ADD . .

# Build
RUN echo Commit: `git log --pretty='%s%b%B' -n 1`
RUN cd $WORKSPACE/cmd/carina-node && go build -ldflags="-X main.gitCommitID=`git rev-parse HEAD`" -gcflags '-N -l' -o /tmp/carina-node .
RUN cd $WORKSPACE/cmd/http-server && go build -gcflags '-N -l' -o /tmp/http-server .

FROM ubuntu:16.04
ENV DEBIAN_FRONTEND=noninteractive

COPY --from=builder /workspace/carina/sources.list /tmp
RUN mv /etc/apt/sources.list /etc/apt/sources.list.bak && mv /tmp/sources.list /etc/apt/ \
    && apt-get update && apt-get -y install --no-install-recommends apt-utils file xfsprogs lvm2 thin-provisioning-tools kmod && rm -rf /var/lib/apt/lists/*
#    && for i in dm_snapshot dm_mirror dm_thin_pool; do modprobe $i; done

# copy binary file
COPY --from=builder /tmp/carina-node /usr/bin/
COPY --from=builder /tmp/http-server /usr/bin/
RUN chmod +x /usr/bin/carina-node
RUN chmod +x /usr/bin/http-server

# Update time zone to Asia-Shanghai
COPY --from=builder /workspace/carina/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' > /etc/timezone

CMD ["http-server"]
