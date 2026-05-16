# Stage 1: build (always runs natively on the build host)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETARCH
ARG APP_VERSION=dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-X main.version=${APP_VERSION}" -o daysuntil .

# Stage 2: run
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/daysuntil .
COPY static/ ./static/
VOLUME /data
ENV DB_PATH=/data/daysuntil.db
EXPOSE 8080
CMD ["./daysuntil"]
