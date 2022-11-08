FROM golang:alpine AS builder

LABEL stage=gobuilder

ENV CGO_ENABLED 0

RUN apk update --no-cache

WORKDIR /build

ADD go.mod .
ADD go.sum .
RUN go mod download
COPY . .
COPY configs /app/configs

RUN go build -ldflags="-s -w" -o /app/dfserver cmd/main.go


FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

WORKDIR /app
COPY --from=builder /app/dfserver /app/dfserver
COPY --from=builder /app/configs /app/configs

CMD ["./dfserver", "--confpath=/app/configs"]
