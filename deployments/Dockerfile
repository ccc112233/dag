FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/dap ./cmd/dap

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/dap /usr/local/bin/dap
COPY configs/example.yaml /etc/dap/config.yaml
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dap","--config","/etc/dap/config.yaml"]
