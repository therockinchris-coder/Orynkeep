FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go test ./... && CGO_ENABLED=0 go build -trimpath -o /server ./cmd/server

FROM alpine:3.22
RUN apk add --no-cache ca-certificates && addgroup -g 10001 app && adduser -D -u 10001 -G app app && mkdir /data && chown app:app /data
COPY --from=build /server /usr/local/bin/orynkeep
USER app
ENV STORAGE_PATH=/data HTTP_ADDR=:8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health/ready || exit 1
CMD ["orynkeep"]
