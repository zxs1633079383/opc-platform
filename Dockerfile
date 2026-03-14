# ---- Build stage ----
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /usr/local/bin/opctl ./cmd/opctl

# ---- Dashboard build stage ----
FROM node:20-alpine AS dashboard-builder

WORKDIR /app
COPY dashboard/package.json dashboard/package-lock.json* ./
RUN npm ci --ignore-scripts 2>/dev/null || npm install

COPY dashboard/ .
RUN npm run build

# ---- Runtime stage ----
FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

COPY --from=builder /usr/local/bin/opctl /usr/local/bin/opctl
COPY --from=dashboard-builder /app/.next/standalone /opt/dashboard/
COPY --from=dashboard-builder /app/.next/static /opt/dashboard/.next/static
COPY --from=dashboard-builder /app/public /opt/dashboard/public

# Default data directory
RUN mkdir -p /data/opc

ENV HOME=/data
ENV OPC_STATE_DIR=/data/opc

EXPOSE 9527 3000

ENTRYPOINT ["opctl"]
CMD ["serve", "--host", "0.0.0.0"]
