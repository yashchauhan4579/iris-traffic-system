# Iris - Intelligent Traffic Management System

Iris is a comprehensive, AI-powered traffic management and analytics platform designed to monitor, analyze, and optimize urban traffic flow. It leverages advanced computer vision to provide real-time insights into vehicle classification, license plate recognition, traffic violations, and crowd density.

## 🚀 Key Features

### 1. **Vehicle Classification & Counting (VCC)**
- **Real-time Counting**: Accurate counting of vehicles across multiple categories (2-Wheeler, 4-Wheeler, Bus, Truck, Auto, HMV).
- **Analytics Dashboard**: Visualizations for hourly/weekly distribution, peak traffic hours, and vehicle composition.
- **Detailed Reports**: Generate PDF and Excel reports with granular data (Timestamp, Confidence, Direction, Lane, etc.).

### 2. **Automatic Number Plate Recognition (ANPR)**
- **Plate Detection**: High-accuracy detection and reading of license plates.
- **Vehicle Identification**: Identifies vehicle make, model, and color along with the plate.
- **Watchlist Alerts**: Real-time alerts for blacklisted or suspicious vehicles [Planned].

### 3. **Traffic Violation Detection**
- **Automated Enforcement**: Detects various violations such as:
    - Speeding
    - Red Light Violation (RLVD)
    - No Helmet / No Seatbelt
    - Wrong Side Driving
- **Evidence generation**: Captures snapshots and video clips for challan generation.

### 4. **Crowd Monitoring**
- **Density Analysis**: Real-time crowd density estimation and heatmap generation.
- **Safety Alerts**: Alerts for overcrowding or abnormal gathering patterns.

### 5. **Reporting & Data Export**
- **PDF Reports**: Professional-grade traffic analysis reports with charts and summary statistics.
- **Excel Export**: Download raw event data (up to 30,000 records) for offline analysis.

## 🛠️ Tech Stack

### Frontend
- **Framework**: [React](https://react.dev/) with [TypeScript](https://www.typescriptlang.org/)
- **Build Tool**: [Vite](https://vitejs.dev/)
- **Styling**: [Tailwind CSS](https://tailwindcss.com/) & [shadcn/ui](https://ui.shadcn.com/)
- **Charts**: [Recharts](https://recharts.org/)
- **PDF Generation**: [@react-pdf/renderer](https://react-pdf.org/)
- **Maps**: [Leaflet](https://leafletjs.com/) / React-Leaflet

### Backend
- **Language**: [Go (Golang)](https://go.dev/)
- **Framework**: [Gin Web Framework](https://gin-gonic.com/)
- **Database**: PostgreSQL with PostGIS extension (via GORM)
- **Containerization**: Docker & Docker Compose

## 📦 Project Structure

```bash
iris/
├── backend/                # Go Backend API
│   ├── cmd/                # Entry points
│   ├── handlers/           # HTTP Request Handlers (VCC, ANPR, etc.)
│   ├── models/             # Database Models (GORM)
│   ├── services/           # Business Logic Services
│   └── Dockerfile          # Backend Docker Config
├── client/                 # React Frontend Application
│   ├── src/
│   │   ├── components/     # UI Components (Dashboards, Reports)
│   │   ├── contexts/       # React Context Providers
│   │   └── lib/            # Utilities & API Client
│   └── vite.config.ts      # Vite Configuration
├── irisv3/                 # Legacy/Alternative Version
├── magicbox-node/          # Edge Compute Node Implementation
└── media/                  # Media Resources
```

## 🚀 Getting Started

### Prerequisites
- [Docker](https://www.docker.com/) & Docker Compose
- [Node.js](https://nodejs.org/) (v18+)
- [Go](https://go.dev/) (v1.21+)

### Running with Docker (Recommended)

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/Nikhilkaushik23/ATCC.git
    cd iris
    ```

2.  **Start the Backend & Database:**
    ```bash
    cd backend
    docker compose up -d --build
    ```
    The backend API will be available at `http://localhost:3001`.

3.  **Start the Frontend (Development Mode):**
    ```bash
    cd ../client
    npm install
    npm run dev
    ```
    The frontend will be available at `http://localhost:8443` (Note: Port 8443 is configured in `vite.config.ts`).

### Running Manually

1.  **Backend:**
    Ensure PostgreSQL is running. Set environment variables in `.env` (or export them).
    ```bash
    cd backend
    go mod download
    go run main.go
    ```
    *   **API Port**: 3001
    *   **NATS Port**: 4233 (Embedded)

2.  **Frontend:**
    ```bash
    cd client
    npm install
    npm run dev
    ```
    *   **Frontend Port**: 8443
    *   **Media Server Proxy**: Requests to `/media` are proxied to `http://localhost:8888`.

## 📄 License
[Provide License Information Here]
