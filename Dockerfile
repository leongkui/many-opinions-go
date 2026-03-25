# Build stage
FROM golang:1.26.1-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o many-opinions-go .

# Run stage
FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/many-opinions-go .
COPY models.json .

EXPOSE 19801

ENTRYPOINT ["./many-opinions-go", "-transport", "sse", "-port", "19801"]
