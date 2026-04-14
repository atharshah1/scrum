FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/. .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/backend ./cmd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /bin/backend /app/backend
COPY --from=builder /bin/worker /app/worker
COPY --from=builder /app/migrations /app/migrations
EXPOSE 8080
ENTRYPOINT ["/app/backend"]
