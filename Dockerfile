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

FROM antmoveh/centos-lvm2:v3

# copy binary file
COPY --from=builder /tmp/carina-node /usr/bin/
COPY --from=builder /tmp/http-server /usr/bin/
COPY --from=builder /workspace/carina/deploy/develop/config.json /etc/carina/

RUN chmod +x /usr/bin/http-server && chmod +x /usr/bin/carina-node

# Update time zone to Asia-Shanghai
COPY --from=builder /workspace/carina/Shanghai /etc/localtime
RUN echo 'Asia/Shanghai' > /etc/timezone

CMD ["http-server"]
