FROM golang:1.17 as builder
ARG TARGETARCH
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -o output/proxyclient node-proxy-client/client.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -a -o output/proxyserver local-proxy-server/server.go

FROM scratch
WORKDIR /root
COPY --from=builder /app/output/proxyclient .
EXPOSE 8888
CMD ["/root/proxyclient"]