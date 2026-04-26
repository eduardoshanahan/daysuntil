# Stage 1: build
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o daysuntil .

# Stage 2: run
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/daysuntil .
COPY static/ ./static/
VOLUME /data
ENV DB_PATH=/data/daysuntil.db
EXPOSE 8080
CMD ["./daysuntil"]
