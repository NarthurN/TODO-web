FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ /app/cmd/
COPY internal/ /app/internal/
COPY pkg/ /app/pkg/
COPY web /app/web
COPY .env .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /my_app ./cmd/app

# Финальный этап
FROM alpine:latest

WORKDIR /app

# Копируем только исполняемый файл и директорию web
COPY --from=builder /my_app /app/my_app
COPY --from=builder /app/web /app/web
COPY --from=builder /app/.env /app/.env

# Указываем порт
EXPOSE 7540

CMD ["/app/my_app"]