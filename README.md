# MiniEdge — Developer Command Center (Frontend)

> **Zero-Dependency Local Developer Gateway and Microservices Command Center**

MiniEdge is a developer monitoring and control console designed to help engineers instantly understand the health, routing, traffic volume, latency, and fault states of multiple local microservices.

---

## ⚡ Key Highlights & Hackathon Compliance

- **100% Zero Runtime Dependencies**: Pure HTML5, CSS3, and modern Vanilla ES6 Modules.
- **Zero Third-Party Libraries**: Custom Canvas/SVG renderers for telemetry charts and topological service map. No component libraries, no chart frameworks, no icon packages.
- **Interactive Mock Traffic Engine**: Background simulator emitting real-time HTTP requests, dynamic latencies, error distributions, and structured logs.
- **Controlled Fault Injection Studio**: Interactive controls to inject latency, simulate 500/503 outages, and restore services with real-time propagation across the entire command center (**LOCAL AND INTENTIONAL**).
- **Future-Proof Data Layer**: Clean decoupled client (`js/api/client.js`) enabling zero-refactor switching to real MiniEdge REST endpoints.

---

## 🚀 How to Run

Because MiniEdge uses standard native ES6 modules, you can serve it with any local static HTTP server or Python:

### Option 1: Python HTTP Server (Built into Windows/macOS/Linux)
```powershell
cd C:\Users\Bhavyadeepsinh\.gemini\antigravity\scratch\miniedge-frontend
python -m http.server 8000
```
Then open `http://localhost:8000` in your web browser.

### Option 2: Node npx (Zero installation)
```powershell
cd C:\Users\Bhavyadeepsinh\.gemini\antigravity\scratch\miniedge-frontend
npx serve .
```

---

## 🛠️ Dashboard Architecture

```
miniedge-frontend/
├── index.html                    # Main HTML shell & container
├── styles/
│   ├── base.css                  # Design tokens, dark theme, typography, buttons
│   ├── layout.css                # Grid structure, navigation header, responsive breakpoints
│   └── components.css            # Component styles (Cards, SVG Map, Canvas, Terminal, Modal)
├── js/
│   ├── app.js                    # Application bootstrapper and keyboard shortcut listener
│   ├── state.js                  # Reactive state store and event emitter
│   ├── api/
│   │   ├── client.js             # Data access layer (Mock vs Real REST API switch)
│   │   └── mockData.js           # Seed data & live traffic generator
│   ├── components/
│   │   ├── header.js             # Header with status badge & refresh controls
│   │   ├── overviewMetrics.js    # KPI cards (Services, Healthy, Total Reqs, Error Rate, Latency)
│   │   ├── serviceCards.js       # Microservice cards (Users, Events, Orders, Payments)
│   │   ├── serviceMap.js         # Interactive SVG service topology graph with animated traffic
│   │   ├── metricsCharts.js      # Zero-dependency Canvas charts (Throughput, Errors, Latency)
│   │   ├── requestExplorer.js    # Filterable request table with detail inspector modal
│   │   ├── logViewer.js          # Streaming terminal log console with level filtering & search
│   │   └── simulationControl.js  # Chaos & fault injection studio
│   └── utils/
│       ├── formatters.js         # Duration, number, and status formatters
│       └── svgIcons.js           # Pure inline SVG definitions
└── README.md
```

---

## 📊 Monitored Services (Default State)

| Service | Port | Default Status | Latency | Endpoints |
| :--- | :--- | :--- | :--- | :--- |
| **Users** | `:3001` | `UP` | ~24ms | `GET /users`, `GET /users/:id`, `POST /users/auth`, `PUT /users/profile` |
| **Events** | `:3002` | `SLOW` | ~820ms | `GET /events`, `POST /events/publish`, `GET /events/stream`, `GET /events/analytics` |
| **Orders** | `:3003` | `DOWN` | 0ms (Timeout) | `GET /orders`, `POST /orders/checkout`, `POST /orders/create`, `GET /orders/:id` |
| **Payments** | `:3004` | `UP` | ~31ms | `POST /payments/charge`, `POST /payments/intent`, `GET /payments/webhook`, `POST /payments/refund` |

---

## ⌨️ Developer Shortcuts

- **`[Space]`**: Pause / Resume live traffic simulation ticker.
- **`[R]`**: Trigger immediate manual health check & traffic refresh.

---

## 🔌 Team Backend Integration Contract

When the backend gateway is ready, update `js/api/client.js`:

```javascript
import { apiClient } from './api/client.js';

// Switch from mock mode to live backend
apiClient.setMode('real');
apiClient.setBaseUrl('http://localhost:8080');
```

Backend REST Endpoints supported by the contract:
- `GET /api/services` — Returns list of microservice health objects.
- `GET /api/services/:id` — Returns single service metrics.
- `GET /api/metrics` — Returns throughput, error count, p50, p95 series.
- `GET /api/logs?level=ERROR&service=orders` — Returns filtered structured logs.
- `GET /api/service-map` — Returns node and edge topology definitions.
- `GET /api/health` — Gateway status check.
- `POST /api/simulations` — Dispatches upstream chaos / latency rules to the gateway.
