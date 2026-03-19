FROM golang:alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY . .

RUN go build -o /event-ingestor ./services/event-ingestor/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /event-ingestor .

EXPOSE 8080
EXPOSE 2112
CMD ["./event-ingestor"]
