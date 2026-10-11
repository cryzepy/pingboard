# ---- Tahap 1: build frontend ----
FROM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- Tahap 2: build backend ----
FROM golang:1.24-alpine AS backend
WORKDIR /src
COPY backend/go.mod ./
COPY backend/*.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pingboard .

# ---- Tahap 3: image akhir (kecil) ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /pingboard /pingboard
COPY --from=frontend /app/dist /static
ENV PORT=8080 STATIC_DIR=/static
EXPOSE 8080
ENTRYPOINT ["/pingboard"]
