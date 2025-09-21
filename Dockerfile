# Development stage (for hot reloading)
FROM golang:alpine AS development

WORKDIR /app

# Install Air for hot reloading
RUN go install github.com/air-verse/air@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

EXPOSE 8080
CMD ["air", "-c", ".air.toml"]

# Production build stage
FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./ 
RUN go mod download
COPY . . 
RUN go build -o server .

# Production run stage
FROM alpine:latest AS production

WORKDIR /app 
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]
