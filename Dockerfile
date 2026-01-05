FROM golang:1.25 AS builder
WORKDIR /src
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/iptv ./cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /bin/iptv /iptv
EXPOSE 8080
ENTRYPOINT ["/iptv"]
