FROM golang:1.26-alpine AS build

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api/

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -s /bin/sh appuser

COPY --from=build /app/bin/api /usr/local/bin/api

USER appuser

EXPOSE 8080

ENTRYPOINT ["api"]
