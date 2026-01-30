# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Iris is a full-stack AI-powered traffic management and analytics platform with:
- **Backend**: Go 1.21 + Gin Framework + PostgreSQL + Embedded NATS
- **Frontend**: React 19 + TypeScript + Vite + Tailwind CSS

Key features: Vehicle Classification & Counting (VCC), ANPR, Traffic Violation Detection, Crowd Density Monitoring, Edge Worker Management.

## Common Commands

### Backend (from `/backend`)
```bash
make run              # Run: go run main.go
make build            # Build binary: go build -o backend main.go
make test             # Run tests: go test ./...
make fmt              # Format code: go fmt ./...
make lint             # Lint (requires golangci-lint)
make deps             # Download dependencies
make seed-violations  # Seed test data
```

### Frontend (from `/client`)
```bash
npm run dev      # Start dev server on port 8443
npm run build    # TypeScript check + Vite build
npm run lint     # ESLint check
npm run preview  # Preview production build
```

### Docker
```bash
cd backend && docker compose up -d --build  # Starts backend + PostgreSQL
```

## Architecture

### Directory Structure
```
iris/
├── backend/
│   ├── handlers/      # HTTP request handlers (auth, vcc, violations, crowd, workers, etc.)
│   ├── models/        # GORM database models
│   ├── services/      # Business logic (feedhub, wireguard)
│   ├── database/      # GORM connection & auto-migration
│   ├── natsserver/    # Embedded NATS setup
│   └── main.go        # Server init, route setup
├── client/
│   ├── src/
│   │   ├── components/   # Feature-based components (anpr/, vcc/, crowd/, violations/, workers/, map/, cameras/, layout/, ui/)
│   │   ├── contexts/     # React Context (Auth, Theme, DeviceFilter, etc.)
│   │   ├── lib/          # API client (api.ts) + utilities
│   │   └── pages/        # Page components
│   └── vite.config.ts    # Vite config with API proxy
```

### API Routes (Base: `http://localhost:3001/api`)
- `/login` - Authentication
- `/devices` - Camera/device management
- `/events` - Event ingestion from workers
- `/workers`, `/admin/workers` - Edge worker management
- `/crowd` - Crowd analysis & alerts
- `/violations` - Traffic violations
- `/vcc` - Vehicle classification & counting
- `/vehicles`, `/watchlist` - Vehicle tracking

### WebSocket
- Endpoint: `ws://localhost:3001/ws/feeds`
- Purpose: Real-time camera feed streaming
- Managed by `FeedHub` service in `services/feedhub.go`

### Database
- PostgreSQL with auto-migration on startup via GORM
- Core models in `models/models.go`: Device, Event, Worker, WorkerToken, CrowdAnalysis, TrafficViolation, Vehicle, VehicleDetection, Watchlist

## Development Setup

### Environment Variables (backend `.env`)
```
DATABASE_URL=postgresql://user:password@localhost:5432/irisdrone?sslmode=disable
PORT=3001
JWT_SECRET=your-secret-key
WIREGUARD_ENDPOINT=localhost:51820
```

### Ports
- Frontend dev: 8443
- Backend API: 3001
- PostgreSQL: 5432
- NATS: 4233

### Default Credentials
- Username: `wiredleap_atcc`
- Password: `wiredleap12**`

## Key Patterns

### Backend
- Gin context for request/response handling
- GORM with auto-migration and query builder pattern
- JWT authentication with HS256 signing (24h expiry)
- Embedded NATS for pub/sub messaging

### Frontend
- React Context API for state management (no Redux)
- Centralized API client in `lib/api.ts` with TypeScript interfaces
- Tailwind CSS + Shadcn/ui components
- React Router v7 with protected routes
- Chart.js/Recharts for analytics, Leaflet/Google Maps for mapping

### API Client
- JWT token injected via request interceptor
- TypeScript interfaces defined for all API responses
- API proxy configured in `vite.config.ts` to route `/api` → backend
