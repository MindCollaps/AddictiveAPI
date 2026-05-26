FROM golang:1.22-alpine AS build

WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20

WORKDIR /app

RUN addgroup -S app && adduser -S app -G app && mkdir -p /app/data && chown -R app:app /app

COPY --from=build /out/api /app/api

USER app

EXPOSE 8080

ENV PORT=8080 \
	DATABASE_PATH=/app/data/app.db \
	APP_ENV=production \

ENTRYPOINT ["/app/api"]