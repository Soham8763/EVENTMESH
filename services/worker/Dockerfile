FROM golang:alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY . .

RUN go build -o /worker ./services/worker/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /worker .

EXPOSE 2114
CMD ["./worker"]
