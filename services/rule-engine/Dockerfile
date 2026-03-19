FROM golang:alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY . .

RUN go build -o /rule-engine ./services/rule-engine/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /rule-engine .

CMD ["./rule-engine"]
