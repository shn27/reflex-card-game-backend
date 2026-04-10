# ---- build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server

# ---- runtime stage ----
FROM scratch

COPY --from=builder /bin/server /server

EXPOSE 8080

ENTRYPOINT ["/server"]