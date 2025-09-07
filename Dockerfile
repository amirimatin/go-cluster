FROM golang:1.23 as build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/clusterctl ./cmd/clusterctl

FROM alpine:3.19
RUN adduser -D -u 10001 app
COPY --from=build /out/clusterctl /usr/local/bin/clusterctl
USER app
ENTRYPOINT ["/usr/local/bin/clusterctl"]
