# Daily Operations Architecture Design

## Problem Statement

The fish fry order system needs to:
1. **Persist all orders forever** for historical exploration and reporting
2. **Operate on a single day's orders** during active workflow
3. Support **end-of-day transitions** cleanly
4. Provide **historical reporting** for admins

## Current State Analysis

### Database Schema (as of Feb 2026)

```sql
CREATE TABLE orders (
    id INTEGER PRIMARY KEY DEFAULT nextval('order_id_seq'),
    vehicle_description TEXT,  -- optional
    status TEXT NOT NULL CHECK (status IN ('NEW', 'IN_PROGRESS', 'COMPLETED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE order_items (
    id TEXT PRIMARY KEY,  -- UUID
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id TEXT NOT NULL REFERENCES menu_items(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE menu_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Current Behavior

| Aspect | Implementation | Limitation |
|--------|---------------|------------|
| Daily order number | Calculated via `ROW_NUMBER() OVER (PARTITION BY DATE(created_at))` | Unstable if orders deleted; recalculates on every query |
| Order fetching | `SELECT * FROM orders` (all orders) | No filtering; loads entire history |
| Price at order time | Looked up from `menu_items` at query time | Revenue changes if prices updated |
| End of day | "Purge" = destructive delete | No historical data |
| Date filtering | Only in stats calculation | No first-class support |

### Key Insight: The Event Date Problem

Fish fry events occur on specific dates (typically Fridays during Lent). The system must distinguish:
- **Event date**: The fish fry being worked (e.g., "Friday March 7, 2026")
- **Created timestamp**: When the order was entered (could be 11:59 PM or 12:01 AM)

This matters because:
- An order entered at 12:01 AM Saturday is still for Friday's fish fry
- Daily order numbers must be per-event, not per-calendar-day
- Historical queries are by event, not by arbitrary date range

---

## Architecture Alternatives

### Option A: Session-Based Architecture

**Concept**: Explicit "sessions" that admins open and close.

```sql
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    event_date DATE NOT NULL UNIQUE,
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    notes TEXT
);

ALTER TABLE orders ADD COLUMN session_id INTEGER REFERENCES sessions(id);
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER NOT NULL;
```

**Workflow**:
1. Admin opens a session for today's event
2. All new orders are assigned to the active session
3. Daily order numbers auto-increment within session
4. Admin closes session at end of night
5. No new orders can be created without an active session

**Pros**:
- Explicit control over session boundaries
- Clean separation between events
- Can handle edge cases (order at 12:01 AM goes to correct session)
- Session metadata (notes, open/close times) useful for auditing

**Cons**:
- Extra step: must "open" session before taking orders
- What if admin forgets to close? (Could auto-close after 24h)
- More complex state management

---

### Option B: Implicit Date-Based Architecture

**Concept**: Use `event_date` field, defaulting to current date, but allowing override.

```sql
ALTER TABLE orders ADD COLUMN event_date DATE NOT NULL DEFAULT CURRENT_DATE;
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER NOT NULL;
```

**Workflow**:
1. Orders automatically get today's date as `event_date`
2. Daily order numbers calculated per `event_date`
3. If working past midnight, UI allows selecting "still working Friday's event"
4. Historical queries filter by `event_date`

**Pros**:
- Simpler than sessions - no explicit open/close
- Automatic for normal cases
- Still handles midnight edge case via UI

**Cons**:
- No explicit "session closed" state
- Can accidentally create orders for wrong date
- No session-level metadata

---

### Option C: Hybrid Approach

**Concept**: Implicit date-based with optional session metadata.

```sql
CREATE TABLE event_sessions (
    event_date DATE PRIMARY KEY,
    status TEXT CHECK (status IN ('ACTIVE', 'CLOSED')) DEFAULT 'ACTIVE',
    opened_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    closed_at TIMESTAMP,
    total_orders INTEGER DEFAULT 0,
    total_revenue DECIMAL(10,2) DEFAULT 0,
    notes TEXT
);

ALTER TABLE orders ADD COLUMN event_date DATE NOT NULL DEFAULT CURRENT_DATE;
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER NOT NULL;
```

**Workflow**:
1. First order of a day auto-creates the session record
2. Orders assigned to current `event_date` (configurable in UI if past midnight)
3. Admin can "close" session to prevent new orders and snapshot stats
4. Closed sessions become read-only for historical browsing

**Pros**:
- Best of both: automatic start, explicit close
- Session stats captured at close time (immutable)
- Flexible for edge cases

**Cons**:
- Slightly more complex than pure date-based
- Need to handle "reopen" if closed accidentally

---

### Option D: Archive-Based Architecture

**Concept**: Keep current schema, add archive flag.

```sql
ALTER TABLE orders ADD COLUMN archived_at TIMESTAMP;
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER;
```

**Workflow**:
1. Active orders: `WHERE archived_at IS NULL`
2. End of day: `UPDATE orders SET archived_at = NOW() WHERE archived_at IS NULL`
3. Historical: `WHERE archived_at IS NOT NULL AND DATE(created_at) = ?`

**Pros**:
- Minimal schema change
- Simple archive/unarchive

**Cons**:
- No explicit event date (still relies on `created_at`)
- Midnight problem not solved
- No session-level stats

---

## Additional Considerations

### Price Capture at Order Time

Regardless of session approach, we should capture prices when ordered:

```sql
ALTER TABLE order_items ADD COLUMN unit_price DECIMAL(10,2) NOT NULL;
ALTER TABLE order_items ADD COLUMN item_name TEXT NOT NULL;
```

**Rationale**:
- Menu prices change between events
- Historical revenue must reflect actual prices charged
- If menu item deleted, order history still readable

### Daily Order Number Storage

Currently calculated dynamically. Should be stored:

```sql
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER NOT NULL;
```

**Assignment logic**:
```sql
-- On insert, calculate next number for this event_date
INSERT INTO orders (event_date, daily_order_number, ...)
VALUES (
    :event_date,
    (SELECT COALESCE(MAX(daily_order_number), 0) + 1
     FROM orders WHERE event_date = :event_date),
    ...
);
```

### Global Order ID vs Daily Order Number

Keep both:
- `id`: Global unique identifier (for database integrity, API references)
- `daily_order_number`: Human-friendly number shown to customers ("Order #7")

---

## Open Questions for Discussion

1. **Session lifecycle**:
   - Should sessions auto-open on first order, or require explicit open?
   - Should closed sessions allow reopening?
   - What happens if someone tries to order when session is closed?

2. **Midnight handling**:
   - Fish fry runs until ~8 PM typically. Is midnight edge case even realistic?
   - If so: UI toggle "Continue previous event" or explicit event_date selector?

3. **Historical access**:
   - Who can view historical data? (Admin only, or workers too?)
   - What historical views are needed? (By date, date range, item, revenue?)

4. **Stats capture**:
   - Capture final stats when session closes? (Immutable snapshot)
   - Or always calculate from order data? (Reflects any corrections)

5. **Order corrections**:
   - Can completed orders be edited after session closes?
   - Should there be an audit log of changes?

6. **Multi-event days**:
   - Ever have two events in one day? (Lunch and dinner?)
   - If so, session-based is better than date-based

---

## Chosen Approach: Auto-Create Sessions with Expiry

### Core Concept

Sessions are created automatically on first order and remain active until their expiry time. This provides:
- **Zero friction** for normal daily operation (just start taking orders)
- **Flexibility** for extended events (weekend festivals, etc.)
- **Clean boundaries** between events

### Session Lifecycle

```
First Order Arrives
        ↓
┌─────────────────────────────────────────┐
│  Session Auto-Created                   │
│  - event_name: "Fish Fry 2026-03-07"    │
│  - started_at: NOW()                    │
│  - expires_at: midnight tonight         │
│  - status: ACTIVE                       │
└─────────────────────────────────────────┘
        ↓
   Orders accumulate (daily_order_number increments)
        ↓
   Admin can extend expires_at if needed
        ↓
┌─────────────────────────────────────────┐
│  Session Expires (or manually closed)   │
│  - status: CLOSED                       │
│  - closed_at: NOW()                     │
│  - Final stats snapshotted              │
└─────────────────────────────────────────┘
        ↓
   Next order auto-creates new session
```

### Handling Edge Cases

| Scenario | Behavior |
|----------|----------|
| Normal Friday fish fry | Session auto-creates, expires at midnight, next Friday gets new session |
| Running late (11:30 PM) | Admin extends expiry to 1:00 AM |
| Weekend festival | Admin sets expiry to Sunday midnight |
| Forgot to extend, expired mid-service | Admin can reopen or create new session |

---

## Final Database Schema

### New Tables

```sql
-- Event sessions (auto-created on first order)
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    event_name TEXT NOT NULL,  -- e.g., "Fish Fry 2026-03-07"
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,  -- Default: midnight of started_at day
    closed_at TIMESTAMP,  -- NULL = still active or expired but not formally closed
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CLOSED')),

    -- Snapshot stats (populated when closed)
    final_order_count INTEGER,
    final_revenue DECIMAL(10,2),

    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for finding active session quickly
CREATE INDEX idx_sessions_status ON sessions(status) WHERE status = 'ACTIVE';
```

### Modified Tables

```sql
-- Orders: add session reference and stored daily number
ALTER TABLE orders ADD COLUMN session_id INTEGER NOT NULL REFERENCES sessions(id);
ALTER TABLE orders ADD COLUMN daily_order_number INTEGER NOT NULL;

-- Order items: capture price and name at order time
ALTER TABLE order_items ADD COLUMN unit_price DECIMAL(10,2) NOT NULL;
ALTER TABLE order_items ADD COLUMN item_name TEXT NOT NULL;
```

### Complete New Schema (for fresh install)

```sql
-- Sessions
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    event_name TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    closed_at TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CLOSED')),
    final_order_count INTEGER,
    final_revenue DECIMAL(10,2),
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Menu items (unchanged)
CREATE TABLE menu_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Orders (updated)
CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id),
    daily_order_number INTEGER NOT NULL,
    vehicle_description TEXT,
    status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'IN_PROGRESS', 'COMPLETED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Ensure unique daily numbers within a session
    UNIQUE (session_id, daily_order_number)
);

-- Order items (updated)
CREATE TABLE order_items (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id TEXT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name TEXT NOT NULL,  -- Captured at order time
    unit_price DECIMAL(10,2) NOT NULL,  -- Captured at order time
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Chat messages (unchanged)
CREATE TABLE chat_messages (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sender_role TEXT NOT NULL CHECK (sender_role IN ('WORKER', 'ADMIN')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_orders_session ON orders(session_id);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_menu_items_display_order ON menu_items(display_order);
```

---

## API Design

### Session Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/session` | Get current active session (or null) |
| POST | `/api/session` | Manually create a session (admin) |
| PUT | `/api/session/:id` | Update session (extend expiry, add notes) |
| POST | `/api/session/:id/close` | Manually close session |
| GET | `/api/sessions` | List all sessions (for history) |
| GET | `/api/sessions/:id` | Get session details with stats |

### Order Endpoints (updated)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/orders` | Get orders for **current active session only** |
| POST | `/api/orders` | Create order (auto-creates session if none active) |
| GET | `/api/orders/:id` | Get specific order |
| PUT | `/api/orders/:id/status` | Update order status |
| GET | `/api/sessions/:id/orders` | Get orders for a specific historical session |

### Session Auto-Creation Logic

```go
func (h *OrderHandler) CreateOrder(c echo.Context) error {
    // 1. Find active session
    session, err := h.repo.GetActiveSession(ctx)

    // 2. If no active session, create one
    if session == nil {
        session = &Session{
            EventName: fmt.Sprintf("Fish Fry %s", time.Now().Format("2006-01-02")),
            StartedAt: time.Now(),
            ExpiresAt: endOfDay(time.Now()),  // Midnight tonight
            Status:    "ACTIVE",
        }
        err = h.repo.CreateSession(ctx, session)
    }

    // 3. Check if session is expired
    if time.Now().After(session.ExpiresAt) {
        return echo.NewHTTPError(http.StatusConflict,
            "Session has expired. Please extend the session or start a new one.")
    }

    // 4. Get next daily order number
    dailyNum, err := h.repo.GetNextDailyOrderNumber(ctx, session.ID)

    // 5. Create order with session reference
    order := &Order{
        SessionID:        session.ID,
        DailyOrderNumber: dailyNum,
        // ... rest of order fields
    }

    // 6. Capture item prices at order time
    for _, item := range req.Items {
        menuItem, _ := h.repo.GetMenuItemByID(ctx, item.MenuItemID)
        orderItem := &OrderItem{
            MenuItemID: item.MenuItemID,
            ItemName:   menuItem.Name,   // Snapshot
            UnitPrice:  menuItem.Price,  // Snapshot
            Quantity:   item.Quantity,
        }
    }
}
```

---

## Frontend Changes

### Admin Panel Additions

1. **Session Status Card** (top of admin page)
   - Shows: "Active Session: Fish Fry 2026-03-07"
   - Expires: "Tonight at 11:59 PM" with countdown
   - Button: "Extend Session" → modal to set new expiry
   - Button: "Close Session" → confirms and closes

2. **Session History Section**
   - List of past sessions with date, order count, revenue
   - Click to drill down into historical orders

### Order Creation Flow

No changes needed - session is auto-created transparently. Workers just create orders as usual.

### Orders Page

- Only shows current session's orders (no change to UX, just filtered data)
- If no active session: "No active session. Create an order to start one."

---

## Migration Path

Since database can be nuked, this is a fresh schema:

1. Drop all existing tables
2. Create new schema with sessions table
3. Seed menu items
4. Ready to go

If we needed to preserve data (future reference):
1. Create sessions table
2. Create one session for all existing orders
3. Add session_id to orders, populate with that session
4. Add daily_order_number, backfill from created_at ordering
5. Add unit_price/item_name to order_items, populate from current menu_items

---

## Design Decisions

### Session Naming
- **Auto-generate**: `"Fish Fry YYYY-MM-DD"` on creation
- **Admin can override**: Rename to `"Lenten Fish Fry Week 3"` or `"Festival Day 1"` etc.

### Session Close Behavior
When admin closes a session:
1. **Snapshot stats** → `final_order_count`, `final_revenue` become immutable
2. **Mark all orders COMPLETED** → No partial/in-progress orders left dangling
3. **Set status to CLOSED** → Prevents new orders
4. **Broadcast to all clients** → Everyone sees session ended

### Expired Session UX
- **Worker sees error**: "Session has expired. Please ask an admin to extend or start a new session."
- **No self-service extend** → Admin controls session lifecycle
- **Clear call to action** → Worker knows exactly what to do

### Historical View
Full-featured historical browsing:
- **Session list** with date, name, order count, revenue
- **Drill-down** into any session → same view as current active session
- **Multi-select / date range** → Compare sessions (e.g., first half vs last half of Lent)
- **Aggregate stats** across selected sessions (total orders, total revenue, by-item breakdown)

---

## Historical Comparison Feature

### Use Cases
- Compare this year's Fish Fry Week 3 to last year's Week 3
- See first half of Lent vs second half
- Year-over-year revenue trends
- Which items sell better early vs late in season

### UI Design

```
┌─────────────────────────────────────────────────────────────┐
│  Session History                                             │
├─────────────────────────────────────────────────────────────┤
│  Filter: [Date Range: Mar 1 - Apr 15] [Year: 2026 ▼]        │
│                                                              │
│  ☑ Fish Fry 2026-03-07    42 orders   $523.50               │
│  ☑ Fish Fry 2026-03-14    38 orders   $478.25               │
│  ☐ Fish Fry 2026-03-21    51 orders   $634.00               │
│  ☐ Fish Fry 2026-03-28    45 orders   $567.75               │
│                                                              │
│  [Compare Selected]  [View Details]                          │
├─────────────────────────────────────────────────────────────┤
│  Selected: 2 sessions | 80 orders | $1,001.75 total         │
│                                                              │
│  Revenue by Item (aggregated):                               │
│  ████████████████░░░░ Fried Fish Dinner    $456.00 (45%)    │
│  ██████████░░░░░░░░░░ Baked Fish Dinner    $312.50 (31%)    │
│  ████░░░░░░░░░░░░░░░░ Kids Pizza           $145.25 (15%)    │
│  ██░░░░░░░░░░░░░░░░░░ Extras               $88.00  (9%)     │
└─────────────────────────────────────────────────────────────┘
```

### API Support

```
GET /api/sessions?from=2026-03-01&to=2026-04-15
    → List sessions in date range

GET /api/sessions/compare?ids=1,2,3
    → Aggregate stats across multiple sessions
    → Returns: total orders, total revenue, by-item breakdown

GET /api/sessions/:id/orders
    → Full order list for drill-down (same as current /api/orders)
```

---

## Updated Schema (Final)

```sql
CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    event_name TEXT NOT NULL,  -- Auto: "Fish Fry YYYY-MM-DD", admin can override
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    closed_at TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'CLOSED')),

    -- Immutable snapshot when closed
    final_order_count INTEGER,
    final_revenue DECIMAL(10,2),

    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE menu_items (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    display_order INTEGER DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id),
    daily_order_number INTEGER NOT NULL,
    vehicle_description TEXT,
    status TEXT NOT NULL DEFAULT 'NEW' CHECK (status IN ('NEW', 'IN_PROGRESS', 'COMPLETED')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (session_id, daily_order_number)
);

CREATE TABLE order_items (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id TEXT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name TEXT NOT NULL,      -- Snapshot at order time
    unit_price DECIMAL(10,2) NOT NULL,  -- Snapshot at order time
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE chat_messages (
    id TEXT PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    sender_role TEXT NOT NULL CHECK (sender_role IN ('WORKER', 'ADMIN')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_started_at ON sessions(started_at);
CREATE INDEX idx_orders_session ON orders(session_id);
CREATE INDEX idx_order_items_order ON order_items(order_id);
CREATE INDEX idx_menu_items_display_order ON menu_items(display_order);
```

---

## Implementation Plan

### Phase 1: Database & Backend Core
1. Create new migration with sessions table
2. Update orders table (add session_id, daily_order_number)
3. Update order_items table (add unit_price, item_name)
4. Implement session repository methods
5. Update order creation to auto-create session
6. Update order creation to capture prices
7. Implement session close (snapshot stats, mark orders complete)

### Phase 2: API Endpoints
1. `GET /api/session` - current active session
2. `PUT /api/session/:id` - update (extend expiry, rename)
3. `POST /api/session/:id/close` - close session
4. `GET /api/sessions` - list with date filtering
5. `GET /api/sessions/:id/orders` - historical order list
6. `GET /api/sessions/compare` - aggregate stats

### Phase 3: Frontend - Active Session
1. Session status card in admin panel
2. Extend expiry modal
3. Close session flow (confirm → close → show summary)
4. Handle expired session error in order form

### Phase 4: Frontend - Historical View
1. Session history list page
2. Multi-select for comparison
3. Date range filter
4. Aggregate stats view with pie chart
5. Drill-down into individual session

---

*Document created: 2026-02-01*
*Last updated: 2026-02-01*
*Decisions finalized: Auto-create sessions, admin-controlled expiry, snapshot on close, full historical comparison*
