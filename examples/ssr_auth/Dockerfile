# Build Stage
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

# Production Stage
FROM scratch
WORKDIR /
COPY --from=builder /app/server /server
EXPOSE 3333
ENTRYPOINT ["/server"]
