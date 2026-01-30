# MagicBox Node

MagicBox Node is the edge worker component for the IRIS platform. It runs on edge devices (like NVIDIA Jetson) to process video streams from cameras and send events to the central IRIS platform.

## Features

- **Embedded Web UI** - Local setup and monitoring interface
- **Platform Registration** - Token-based or approval-based registration
- **Camera Management** - RTSP stream processing with H264/H265 support
- **Event Queue** - File-based queue with retry logic for unreliable networks
- **Analytics Integration** - Connects to Python-based analytics services (ANPR, VCC, Crowd)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    MagicBox Node                        │
│                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐ │
│  │   Web UI    │  │   Config    │  │  Platform Client│ │
│  │  (Gin/HTML) │  │   Manager   │  │  (HTTP/Heartbeat)│ │
│  └─────────────┘  └─────────────┘  └─────────────────┘ │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Camera Manager                      │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐           │   │
│  │  │ Stream 1│ │ Stream 2│ │ Stream N│  (RTSP)   │   │
│  │  └─────────┘ └─────────┘ └─────────┘           │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
│                          ▼                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │           Analytics Services (Python)            │   │
│  │  ┌─────────┐ ┌─────────┐ ┌─────────┐           │   │
│  │  │  ANPR   │ │   VCC   │ │  Crowd  │           │   │
│  │  └─────────┘ └─────────┘ └─────────┘           │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
│                          ▼                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Event Queue (File-based)            │   │
│  │         /data/events/{pending,sent,failed}       │   │
│  └─────────────────────────────────────────────────┘   │
│                          │                              │
└──────────────────────────│──────────────────────────────┘
                           │
                           ▼ HTTP POST
                   ┌───────────────┐
                   │ IRIS Platform │
                   └───────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21+
- Linux (tested on Ubuntu 22.04, Jetson JetPack 5.x)

### Build

```bash
# Build for current platform
make build

# Build for Jetson (ARM64)
make build-jetson
```

### Run

```bash
# Run with default settings
./build/magicbox

# Run with custom settings
./build/magicbox \
  -config /etc/magicbox/config.json \
  -data /var/lib/magicbox \
  -port 8080
```

### Access Web UI

Open http://localhost:8080 in your browser.

## Configuration

Configuration is stored in JSON format:

```json
{
  "nodeName": "magicbox-001",
  "nodeModel": "NVIDIA Jetson Nano",
  "mac": "aa:bb:cc:dd:ee:ff",
  "state": "active",
  "platform": {
    "serverUrl": "https://iris.example.com",
    "workerId": "uuid",
    "authToken": "jwt-token"
  },
  "cameras": [
    {
      "deviceId": "cam-001",
      "name": "Front Gate",
      "rtspUrl": "rtsp://admin:pass@192.168.1.100:554/stream1",
      "analytics": ["anpr", "vcc"],
      "fps": 15,
      "resolution": "1080p",
      "enabled": true
    }
  ]
}
```

## Event Queue

Events are stored as JSON files in directories:

```
/data/events/
├── pending/          # Events waiting to be sent
│   └── {event-id}/
│       ├── event.json
│       └── frame_001.jpg
├── sent/             # Successfully sent events
└── failed/           # Events that failed after max retries
```

Each event has this structure:

```json
{
  "id": "uuid",
  "type": "anpr",
  "deviceId": "cam-001",
  "timestamp": "2025-01-01T12:00:00Z",
  "data": {
    "plateNumber": "KA01AB1234",
    "confidence": 0.95,
    "vehicleType": "4W"
  },
  "images": ["frame_001.jpg"],
  "status": "pending",
  "retries": 0
}
```

## API Endpoints

### Status
- `GET /api/status` - Node status and stats
- `GET /api/resources` - System resources (CPU, memory, GPU)

### Registration
- `POST /api/register` - Register with token
- `POST /api/request-approval` - Request approval (tokenless)
- `GET /api/approval-status` - Check approval status

### Config
- `GET /api/config` - Get current config
- `POST /api/sync` - Sync config from platform

### Queue
- `GET /api/queue/stats` - Queue statistics
- `GET /api/queue/pending` - List pending events
- `POST /api/queue/retry/:id` - Retry a failed event
- `POST /api/queue/retry-all` - Retry all failed events

## Deployment

### Systemd Service

```bash
# Install service
sudo make install-service

# Enable and start
sudo systemctl enable magicbox
sudo systemctl start magicbox

# Check status
sudo systemctl status magicbox
```

### Docker (optional)

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN make build

FROM alpine:latest
COPY --from=builder /app/build/magicbox /usr/local/bin/
ENTRYPOINT ["magicbox"]
```

## Development

```bash
# Install dependencies
make deps

# Run in development mode (with hot reload)
make dev

# Run tests
make test

# Format code
make fmt
```

## License

Proprietary - IRIS Platform

