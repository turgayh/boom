FROM golang:1.24.3-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /boom ./cmd/main.go

FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /boom /boom

EXPOSE 8080

CMD ["/boom"]
