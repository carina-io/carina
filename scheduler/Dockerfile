# Build context is the repo root: the scheduler imports ../api through a go.mod replace
FROM golang:1.26 AS builder

WORKDIR /workspace
COPY go.mod go.sum ./
COPY scheduler/go.mod scheduler/go.sum ./scheduler/
RUN cd scheduler && go mod download
COPY . .

RUN cd scheduler/cmd && CGO_ENABLED=0 go build -o /tmp/carina-scheduler .

FROM alpine:3.23

COPY --from=builder /workspace/scheduler/debug/scheduler-config.yaml /etc/kubernetes
COPY --from=builder /workspace/scheduler/config.json /etc/carina/
COPY --from=builder /tmp/carina-scheduler /bin/carina-scheduler

CMD ["carina-scheduler"]
