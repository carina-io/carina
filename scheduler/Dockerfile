FROM golang:1.15-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/carina/scheduler

WORKDIR $WORKSPACE
ADD . .

RUN cd $WORKSPACE/cmd && go build -gcflags '-N -l' -o /tmp/carina-scheduler .

FROM alpine:3.12

COPY --from=builder /workspace/carina/scheduler/config.json /etc/carina/
COPY --from=builder /tmp/carina-scheduler /bin/carina-scheduler
RUN chmod +x /bin/carina-scheduler

WORKDIR /bin
CMD ["carina-scheduler"]