FROM golang:1.16.6-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/github.com/bocloud/carina/scheduler

WORKDIR $WORKSPACE
ADD . .

RUN cd $WORKSPACE/cmd && go build -gcflags '-N -l' -o /tmp/carina-scheduler .

FROM alpine:3.12
ENV WORKSPACE=/workspace/github.com/bocloud/carina/scheduler

COPY --from=builder $WORKSPACE/debug/scheduler-config.yaml /etc/kubernetes
COPY --from=builder $WORKSPACE/config.json /etc/carina/
COPY --from=builder /tmp/carina-scheduler /bin/carina-scheduler
RUN chmod +x /bin/carina-scheduler

WORKDIR /bin
CMD ["carina-scheduler"]