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

FROM ubuntu:20.04
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /tmp/carina-node /usr/bin/
COPY --from=builder /tmp/http-server /usr/bin/

RUN chmod +x /usr/bin/carina-node
RUN chmod +x /usr/bin/http-server

#Update time zone to Asia-Shanghai
COPY --from=builder /workspace/carina/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' > /etc/timezone

CMD ["http-server"]
