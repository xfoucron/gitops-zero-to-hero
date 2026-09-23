FROM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/backend ./cmd/main.go

FROM gcr.io/distroless/static-debian13:nonroot

WORKDIR /app
COPY --from=builder /out/backend ./backend
COPY --from=builder /src/migrations ./migrations

EXPOSE 80

ENTRYPOINT ["./backend"]
