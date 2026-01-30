# MagicNetwork - WireGuard VPN Server

Standalone WireGuard VPN server for IRIS MagicBox nodes. Provides a REST API for registering and managing VPN peers.

## Features

- 🔐 WireGuard-based VPN server
- 🌐 REST API for peer management
- 🔑 API key authentication
- 📊 Peer status monitoring
- 💾 Persistent peer storage

## Requirements

- Linux with WireGuard installed (`apt install wireguard wireguard-tools`)
- Go 1.21+ (for building)
- Root access (WireGuard requires root)

## Quick Start

```bash
# 1. Install WireGuard
sudo apt update && sudo apt install -y wireguard wireguard-tools

# 2. Build
go build -o magicnetwork ./cmd/magicnetwork

# 3. Generate API key
./magicnetwork --gen-key

# 4. Configure
cp .env.example .env
# Edit .env and set your API key

# 5. Run (as root)
sudo ./start.sh
```

## API Endpoints

### Public (no auth required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/api/info` | Server public key and port |

### Protected (requires API key)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/status` | Server status |
| GET | `/api/peers` | List all peers |
| POST | `/api/peers` | Register new peer |
| GET | `/api/peers/:pubkey` | Get peer details |
| DELETE | `/api/peers/:pubkey` | Remove peer |

### Authentication

Include API key in header:
```
Authorization: Bearer mn_your_api_key_here
```
or
```
X-API-Key: mn_your_api_key_here
```

### Register Peer Example

```bash
curl -X POST http://localhost:8080/api/peers \
  -H "Authorization: Bearer mn_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "id": "magicbox_001",
    "name": "Warehouse MagicBox",
    "public_key": "BASE64_PUBLIC_KEY_HERE"
  }'
```

Response:
```json
{
  "status": "ok",
  "peer": {
    "id": "magicbox_001",
    "name": "Warehouse MagicBox",
    "assigned_ip": "10.10.0.2/24",
    "allowed_ips": "10.10.0.2/32"
  },
  "server": {
    "public_key": "SERVER_PUBLIC_KEY",
    "listen_port": 51820,
    "server_ip": "10.10.0.1"
  }
}
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `MAGICNETWORK_API_KEY` | (generated) | API key for authentication |
| `API_PORT` | 8080 | API server port |
| `WG_PORT` | 51820 | WireGuard listen port |
| `WG_ADDRESS` | 10.10.0.1/24 | Server VPN address |
| `DATA_DIR` | ./data | Data directory |

## Network Setup

After starting MagicNetwork, ensure:

1. **Firewall**: Open UDP port 51820 for WireGuard
2. **IP Forwarding**: Enable if routing between clients
   ```bash
   echo "net.ipv4.ip_forward = 1" | sudo tee /etc/sysctl.d/99-wireguard.conf
   sudo sysctl -p /etc/sysctl.d/99-wireguard.conf
   ```

## Integration with IRIS Platform

Set the following environment variable in your IRIS backend:
```bash
WIREGUARD_ENDPOINT=your-server-ip:51820
MAGICNETWORK_URL=http://localhost:8080
MAGICNETWORK_API_KEY=mn_your_api_key
```

Then update the backend to call MagicNetwork API instead of managing WireGuard directly.

## License

MIT

