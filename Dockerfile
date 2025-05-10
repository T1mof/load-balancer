FROM golang:1.21-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o loadbalancer ./cmd/loadbalancer

FROM alpine:latest

WORKDIR /app

RUN apk --repository http://dl-4.alpinelinux.org/alpine/v3.21/main --repository http://dl-4.alpinelinux.org/alpine/v3.21/community --no-cache add ca-certificates tzdata

COPY --from=builder /app/loadbalancer .
COPY config.yaml .
COPY ./backend /app/backend

ENV TZ=Europe/Moscow

EXPOSE 8080

CMD ["./loadbalancer", "--config=config.yaml"]
