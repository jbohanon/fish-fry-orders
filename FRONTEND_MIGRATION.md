# Frontend Migration Plan: Go Monolith to Separated Frontend/Backend

## Overview

This document outlines a migration from the current Go server (serving both UI templates and API) to a separated architecture with:
- **Backend**: Go API server (JSON REST + WebSocket)
- **Frontend**: React SPA built with Vite, served as static files

## Current Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Go Server (Echo)                     │
│  ┌─────────────────┐  ┌─────────────────────────────┐  │
│  │  Static Files   │  │     API Endpoints           │  │
│  │  /static/*      │  │  /api/orders, /api/auth/*   │  │
│  ├─────────────────┤  │  /ws/orders (WebSocket)     │  │
│  │  HTML Templates │  │                             │  │
│  │  (SSR via Go)   │  │                             │  │
│  └─────────────────┘  └─────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## Target Architecture

```
┌──────────────────────────┐     ┌──────────────────────────┐
│   Frontend (Static)      │     │   Backend (Go API)       │
│   ─────────────────      │     │   ──────────────────     │
│   React + Vite           │     │   /api/orders            │
│   Served from CDN/Nginx  │────▶│   /api/auth/*            │
│   or separate container  │     │   /ws/orders             │
│                          │     │                          │
│   Routes:                │     │   Routes:                │
│   /*  (client-side)      │     │   /api/*                 │
│                          │     │   /ws/*                  │
└──────────────────────────┘     └──────────────────────────┘
```

## Routing Strategy Options

### Option A: Path-Based Routing (Recommended)

Use Gloo Edge to route by path prefix:

| Path Pattern | Destination | Notes |
|--------------|-------------|-------|
| `/api/*` | Go backend | REST API endpoints |
| `/ws/*` | Go backend | WebSocket connections |
| `/*` | Frontend (static) | React SPA, fallback to index.html |

**Gloo VirtualService Configuration:**

```yaml
apiVersion: gateway.solo.io/v1
kind: VirtualService
metadata:
  name: fish-fry-orders
  namespace: gloo-system
spec:
  virtualHost:
    domains:
    - "orders.example.com"
    routes:
    # API routes -> Go backend
    - matchers:
      - prefix: /api/
      routeAction:
        single:
          upstream:
            name: fish-fry-api
            namespace: gloo-system
    
    # WebSocket routes -> Go backend
    - matchers:
      - prefix: /ws/
      routeAction:
        single:
          upstream:
            name: fish-fry-api
            namespace: gloo-system
      options:
        timeout: 0s
        upgrades:
        - websocket: {}
    
    # All other routes -> Frontend static files
    - matchers:
      - prefix: /
      routeAction:
        single:
          upstream:
            name: fish-fry-frontend
            namespace: gloo-system
```

**Pros:**
- Single domain, no CORS issues
- Simpler client-side code (relative URLs work)
- Session cookies work seamlessly

### Option B: Subdomain-Based Routing

| Domain | Destination |
|--------|-------------|
| `orders.example.com` | Frontend (React SPA) |
| `api.orders.example.com` | Go backend |

**Requires:**
- Separate VirtualService for each host
- CORS configuration on Go backend
- Frontend configured to call `api.orders.example.com`

**Pros:**
- Complete separation
- Can scale/deploy independently

**Cons:**
- CORS complexity
- Cookie domain configuration needed

---

## Migration Steps

### Phase 1: Prepare Go Backend for API-Only Mode

#### 1.1 Extract API Routes

Current routes to keep in Go backend:

```go
// API endpoints (keep these)
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/orders
POST   /api/orders
GET    /api/orders/:id
PUT    /api/orders/:id/status
DELETE /api/orders/purge
GET    /api/menu

// WebSocket (keep this)
GET    /ws/orders
```

#### 1.2 Remove Template Rendering

Files to modify or remove:
- `internal/ui/template.go` - Remove or deprecate
- `ui/templates/*.gohtml` - Will be replaced by React components

#### 1.3 Add CORS Support (if using subdomain routing)

```go
import "github.com/labstack/echo/v4/middleware"

e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
    AllowOrigins:     []string{"https://orders.example.com"},
    AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
    AllowCredentials: true,
}))
```

#### 1.4 Update Authentication

Current auth uses session cookies. For path-based routing, this continues to work.
For subdomain routing, ensure cookie domain is set:

```go
// Set cookie domain to parent domain
cookie.Domain = ".example.com"
```

---

### Phase 2: Create React Frontend

#### 2.1 Initialize Vite Project

```bash
# From repository root
npm create vite@latest frontend -- --template react-ts
cd frontend
npm install
```

#### 2.2 Install Dependencies

```bash
npm install react-router-dom  # Client-side routing
npm install @tanstack/react-query  # Data fetching (optional but recommended)
```

#### 2.3 Project Structure

```
frontend/
├── public/
│   └── images/
│       ├── favicon.ico
│       ├── fish-fry-logo.jpg
│       └── fish-fry-logo-trans.png
├── src/
│   ├── api/
│   │   ├── client.ts       # Fetch wrapper
│   │   ├── orders.ts       # Order API calls
│   │   ├── auth.ts         # Auth API calls
│   │   └── menu.ts         # Menu API calls
│   ├── components/
│   │   ├── Layout.tsx      # Nav, footer
│   │   ├── OrderCard.tsx
│   │   ├── OrderForm.tsx
│   │   ├── OrderTable.tsx
│   │   └── StatusBadge.tsx
│   ├── hooks/
│   │   ├── useOrders.ts    # Order data hook
│   │   ├── useWebSocket.ts # WebSocket hook
│   │   └── useAuth.ts      # Auth state hook
│   ├── pages/
│   │   ├── LoginPage.tsx
│   │   ├── OrdersPage.tsx
│   │   ├── NewOrderPage.tsx
│   │   ├── OrderDetailsPage.tsx
│   │   └── AdminPage.tsx
│   ├── types/
│   │   └── index.ts        # TypeScript interfaces
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
├── index.html
├── package.json
├── tsconfig.json
└── vite.config.ts
```

#### 2.4 API Client

```typescript
// src/api/client.ts
const API_BASE = import.meta.env.VITE_API_BASE || '';

export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    credentials: 'include', // Include cookies
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    throw new Error(`API Error: ${response.status}`);
  }

  return response.json();
}
```

#### 2.5 WebSocket Hook

```typescript
// src/hooks/useWebSocket.ts
import { useEffect, useRef, useCallback } from 'react';

export function useOrdersWebSocket(onMessage: (data: any) => void) {
  const wsRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/orders`;
    
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      onMessage(data);
    };

    ws.onclose = () => {
      // Reconnect logic
      setTimeout(() => window.location.reload(), 5000);
    };

    return () => {
      ws.close();
    };
  }, [onMessage]);

  return wsRef;
}
```

#### 2.6 Type Definitions

```typescript
// src/types/index.ts
export interface Order {
  id: number;
  customerName: string;
  vehicle_description: string;
  status: 'new' | 'in-progress' | 'completed';
  items: OrderItem[];
  total: number;
  created_at: string;
  updated_at: string;
}

export interface OrderItem {
  id: string;
  menuItemId: string;
  menuItemName: string;
  price: number;
  quantity: number;
}

export interface MenuItem {
  id: string;
  name: string;
  price: number;
  display_order: number;
}

export interface Stats {
  totalOrders: number;
  ordersToday: number;
  revenue: number;
}
```

#### 2.7 Vite Configuration

```typescript
// vite.config.ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // Proxy API calls to Go backend during development
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
});
```

---

### Phase 3: Component Migration Map

Map existing Go templates to React components:

| Go Template | React Component | Notes |
|-------------|-----------------|-------|
| `base.gohtml` | `Layout.tsx` | Nav, container structure |
| `auth.gohtml` | `LoginPage.tsx` | Login form |
| `orders.gohtml` | `OrdersPage.tsx` + `OrderTable.tsx` | Orders list view |
| `neworder.gohtml` | `NewOrderPage.tsx` + `OrderForm.tsx` | Order creation |
| `orderdetails.gohtml` | `OrderDetailsPage.tsx` | Single order view |
| `admin.gohtml` | `AdminPage.tsx` | Admin dashboard |

JavaScript files to convert:

| Vanilla JS | React Hook/Component |
|------------|----------------------|
| `orders.js` | `useOrders.ts`, `useWebSocket.ts` |
| `neworder.js` | `OrderForm.tsx` component logic |
| `auth.js` | `useAuth.ts` hook |
| `admin.js` | `AdminPage.tsx` component logic |
| `websocket.js` | `useWebSocket.ts` hook |

---

### Phase 4: Deployment

#### 4.1 Frontend Build

```bash
cd frontend
npm run build
# Output in frontend/dist/
```

#### 4.2 Frontend Dockerfile

```dockerfile
# frontend/Dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

#### 4.3 Frontend Nginx Config

```nginx
# frontend/nginx.conf
server {
    listen 80;
    root /usr/share/nginx/html;
    index index.html;

    # SPA fallback - serve index.html for all routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location /assets/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

#### 4.4 Backend Dockerfile Update

```dockerfile
# Dockerfile (backend only - simplified)
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /fish-fry-api ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /fish-fry-api /fish-fry-api
EXPOSE 8080
CMD ["/fish-fry-api"]
```

#### 4.5 Kubernetes/Helm Updates

Create separate deployments:

```yaml
# Backend deployment (existing, modified)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fish-fry-api
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: api
        image: fish-fry-api:latest
        ports:
        - containerPort: 8080
---
# Frontend deployment (new)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fish-fry-frontend
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: frontend
        image: fish-fry-frontend:latest
        ports:
        - containerPort: 80
```

---

### Phase 5: Testing Strategy

#### 5.1 Run Both in Parallel

During migration, run both versions:
- Old: `https://orders.example.com` (current Go monolith)
- New: `https://orders-beta.example.com` (new separated architecture)

#### 5.2 Feature Parity Checklist

```markdown
- [ ] Login/logout functionality
- [ ] View orders list
- [ ] Create new order
- [ ] View order details
- [ ] Update order status
- [ ] Real-time WebSocket updates
- [ ] Admin dashboard (if admin role)
- [ ] Purge orders (admin)
- [ ] Mobile responsive layout
```

---

## Alternative: HTMX Enhancement (Lower Effort)

If full separation is overkill, consider enhancing current Go templates with HTMX:

```html
<!-- Keep Go templates, add HTMX for dynamic updates -->
<script src="https://unpkg.com/htmx.org@1.9"></script>

<table hx-get="/api/orders" 
       hx-trigger="every 5s" 
       hx-swap="innerHTML">
  <!-- Order rows -->
</table>

<form hx-post="/api/orders" 
      hx-target="#orders-table"
      hx-swap="beforeend">
  <!-- Form fields -->
</form>
```

**Pros:**
- Minimal code changes
- Keep server-side rendering
- No separate frontend deployment

**Cons:**
- Still coupled to Go server
- Less flexible than full SPA

---

## Execution Order for AI Agent

```
1. BACKEND_PREP
   - Add CORS middleware if needed
   - Ensure all API endpoints return JSON (verify no HTML rendering on /api/* routes)
   - Add health check endpoint: GET /health -> {"status": "ok"}
   - Test API endpoints work independently

2. FRONTEND_INIT
   - Create frontend/ directory
   - Initialize Vite + React + TypeScript project
   - Install dependencies: react-router-dom
   - Copy static assets from ui/static/images/ to frontend/public/

3. FRONTEND_TYPES
   - Create src/types/index.ts with TypeScript interfaces
   - Match types to existing Go API response structures

4. FRONTEND_API
   - Create src/api/client.ts (fetch wrapper)
   - Create src/api/orders.ts (order CRUD functions)
   - Create src/api/auth.ts (login/logout functions)
   - Create src/api/menu.ts (menu fetch function)

5. FRONTEND_HOOKS
   - Create src/hooks/useAuth.ts
   - Create src/hooks/useOrders.ts  
   - Create src/hooks/useWebSocket.ts

6. FRONTEND_COMPONENTS
   - Create src/components/Layout.tsx (from base.gohtml)
   - Create src/components/StatusBadge.tsx
   - Create src/components/OrderTable.tsx
   - Create src/components/OrderForm.tsx

7. FRONTEND_PAGES
   - Create src/pages/LoginPage.tsx (from auth.gohtml + auth.js)
   - Create src/pages/OrdersPage.tsx (from orders.gohtml + orders.js)
   - Create src/pages/NewOrderPage.tsx (from neworder.gohtml + neworder.js)
   - Create src/pages/OrderDetailsPage.tsx (from orderdetails.gohtml + orderdetails.js)
   - Create src/pages/AdminPage.tsx (from admin.gohtml + admin.js)

8. FRONTEND_ROUTING
   - Create src/App.tsx with react-router routes
   - Implement auth-protected routes

9. FRONTEND_STYLES
   - Migrate ui/static/styles/styles.css to frontend/src/index.css
   - Adjust for React component structure

10. FRONTEND_BUILD
    - Configure vite.config.ts with proxy for dev
    - Test npm run build produces valid dist/

11. DEPLOYMENT_CONFIG
    - Create frontend/Dockerfile
    - Create frontend/nginx.conf
    - Update helm chart with frontend deployment
    - Update Gloo VirtualService for path-based routing

12. TESTING
    - Verify all API calls work through new frontend
    - Verify WebSocket connection establishes
    - Verify auth flow (login, session, logout)
    - Test on mobile viewport

13. CLEANUP (after validation)
    - Remove ui/templates/ directory
    - Remove ui/static/ directory
    - Remove internal/ui/ package
    - Remove template-related routes from Go server
```

---

## Decision Points

Before proceeding, decide:

1. **Routing strategy**: Path-based (recommended) or subdomain?
  - Path-based
2. **State management**: React Query, Zustand, or just useState/useEffect?
  - Native useState if that is sufficient
3. **Styling**: Keep existing CSS, use Tailwind, or CSS-in-JS?
  - Tailwind
4. **TypeScript**: Yes (recommended) or JavaScript?
  - Yes typescript
5. **Testing**: Add Jest/Vitest or skip for MVP?
  - You can add it as long as it doesn't interfere with the MVP.
