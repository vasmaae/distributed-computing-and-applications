FROM golang:1.27-alpine AS builder

WORKDIR /workspace

COPY common/go.mod common/go.sum ./common/
COPY reports/go.mod reports/go.sum ./reports/
WORKDIR /workspace/reports
RUN go mod download

COPY common ../common
COPY reports .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api


FROM alpine:3.24

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=builder /app/bin/api /app/api

EXPOSE 8080

CMD ["/app/api"]