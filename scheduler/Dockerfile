FROM golang:1.15-buster AS builder

ENV GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=https://goproxy.cn,direct
ENV WORKSPACE=/workspace/carina/schedule

WORKDIR $WORKSPACE
ADD . .

RUN cd $WORKSPACE/cmd && go build -gcflags '-N -l' -o /tmp/carian-scheduler .

FROM alpine:3.12

COPY --from=0 /tmp/carian-scheduler /bin/carian-scheduler
RUN chmod +x /bin/carian-scheduler

WORKDIR /bin
CMD ["carian-scheduler"]