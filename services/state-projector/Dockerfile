FROM golang:alpine AS builder

WORKDIR /app

COPY go.work go.work.sum ./
COPY . .

RUN go build -o /state-projector ./services/state-projector/cmd/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /state-projector .

EXPOSE 2115
CMD ["./state-projector"]
