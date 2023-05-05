FROM golang:1.19.0-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/github.com/carina-io/carina/scheduler
ENV GOMODCACHE=$WORKSPACE/vendor

WORKDIR $WORKSPACE
ADD . .

RUN cd $WORKSPACE/cmd && go build -gcflags '-N -l' -o /tmp/carina-scheduler .

FROM alpine:3.16
ENV WORKSPACE=/workspace/github.com/carina-io/carina/scheduler

COPY --from=builder $WORKSPACE/debug/scheduler-config.yaml /etc/kubernetes
COPY --from=builder $WORKSPACE/config.json /etc/carina/
COPY --from=builder /tmp/carina-scheduler /bin/carina-scheduler
RUN chmod +x /bin/carina-scheduler

WORKDIR /bin
CMD ["carina-scheduler"]