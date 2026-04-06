FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
ENV GOTOOLCHAIN=auto
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /boom ./cmd/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /boom /boom
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

CMD ["/boom"]
