FROM golang:alpine AS builder

WORKDIR /app

# Copy the workspace configuration and shared modules first
COPY go.work go.work.sum ./
# Copy everything for workspace build
COPY . .

# Build the binary
RUN go build -o /auth-service ./services/auth-service/cmd/main.go

# Use a minimal alpine image for the final stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /auth-service .

EXPOSE 8081
CMD ["./auth-service"]
