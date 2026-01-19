# IrisDrone Backend (Go)

This is the Go backend for the IrisDrone project, converted from the Node.js/TypeScript server.

## Features

- RESTful API endpoints for devices, crowd analysis, alerts, and workers
- PostgreSQL database using GORM
- CORS enabled
- Static file serving for heatmaps
- Health check endpoint

## Prerequisites

- Go 1.21 or higher
- PostgreSQL database (same as Node.js server)
- Environment variables configured

## Setup

1. Install dependencies:
```bash
go mod download
```

2. Create a `.env` file (copy from `.env.example`):
```bash
cp .env.example .env
```

3. Update `.env` with your database connection string:
```
DATABASE_URL=postgresql://user:password@localhost:5432/irisdrone?sslmode=disable
PORT=3001
ENV=development
```

4. Run the server:
```bash
go run main.go
```

Or build and run:
```bash
go build -o backend
./backend
```

## API Endpoints

All endpoints match the Node.js server:

### Devices
- `GET /api/devices` - List all devices
- `GET /api/devices/:id/latest` - Get latest event for a device
- `GET /api/devices/analytics/surges` - Get devices with high risk level

### Ingest
- `POST /api/ingest` - Receive raw event data

### Workers
- `GET /api/workers/config` - Get active devices and their analytics config
- `POST /api/workers/heartbeat` - Worker check-in

### Crowd
- `POST /api/crowd/analysis` - Ingest real-time crowd analysis data
- `GET /api/crowd/analysis` - Get crowd analysis data
- `GET /api/crowd/analysis/latest` - Get latest analysis for devices
- `POST /api/crowd/alerts` - Create a crowd alert
- `GET /api/crowd/alerts` - Get crowd alerts
- `PATCH /api/crowd/alerts/:id/resolve` - Resolve an alert
- `GET /api/crowd/hotspots` - Get current hotspots for map visualization

### Health
- `GET /health` - Health check endpoint

## Database

The backend uses GORM for database operations. The models are automatically migrated on startup. The database schema matches the Prisma schema from the Node.js server.

## Differences from Node.js Server

- Uses GORM instead of Prisma (Prisma Go is less mature)
- Uses Gin framework instead of Express/Fastify
- JSONB fields are handled using a custom JSONB type
- BigInt IDs are handled as int64 (GORM limitation)
- Static file serving for heatmaps is implemented using Gin's static file handler

## Development

The server will automatically connect to the database and run migrations on startup. Make sure your PostgreSQL database is running and accessible.

