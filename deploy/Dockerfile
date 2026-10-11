FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npx vite build

FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist internal/api/dist
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /starfleet ./cmd/starfleet/

FROM scratch
COPY --from=builder /starfleet /starfleet
COPY configs/starfleet.yml /etc/starfleet/starfleet.yml
EXPOSE 8080
ENTRYPOINT ["/starfleet"]
CMD ["-config", "/etc/starfleet/starfleet.yml"]