FROM golang:1.24.3-alpine3.21 AS builder
WORKDIR /build
COPY . .
RUN go build -o files.lee.io

FROM alpine:3.21
WORKDIR /app
COPY --from=builder /build/files.lee.io .
COPY static static
ENTRYPOINT [ "/app/files.lee.io" ]