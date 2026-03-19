FROM golang:alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY . .

RUN go build -o /workflow-orchestrator ./services/workflow-orchestrator/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /workflow-orchestrator .

EXPOSE 2113
CMD ["./workflow-orchestrator"]
