# Aero Gateway

A high-performance, programmable HTTP reverse proxy with a **WebAssembly plugin system**. Custom filter logic is written in Rust, compiled to `.wasm`, and executed inside the Go host at request time — with zero restarts required to swap plugins.

---

## Architecture

```
Internet
   │
   ▼
┌──────────────────────────────────────┐
│  Go Proxy  :8080                     │
│                                      │
│  ┌──────────┐    ┌────────────────┐  │
│  │Middleware│───▶│  Wasm Runtime  │  │
│  │          │    │  (Wazero)      │  │
│  └──────────┘    │                │  │
│       │          │  ┌──────────┐  │  │
│       │          │  │ Instance │  │  │
│       │          │  │ Pool ×32 │  │  │
│       │          │  └──────────┘  │  │
│       │          └────────────────┘  │
│       ▼                              │
│  Upstream :9090                      │
└──────────────────────────────────────┘

┌──────────────────────────────────────┐
│  Go Admin  :8081                     │
│  POST /admin/upload  (hot swap)      │
│  GET  /admin/metrics                 │
└──────────────────────────────────────┘

┌──────────────────────────────────────┐
│  Next.js Dashboard  :3000            │
│  Plugin upload · Live metrics        │
└──────────────────────────────────────┘
```

Each incoming request passes through a Rust filter compiled to WebAssembly. The filter inspects the request and returns a decision — **allow** or **block** — before the request is forwarded upstream. A new filter can be uploaded and activated at runtime via the dashboard without restarting the proxy.

---

## Stack

| Layer | Technology |
|---|---|
| Proxy host | Go, `net/http/httputil` |
| Wasm runtime | [Wazero](https://github.com/tetratelabs/wazero) (pure-Go, no CGO) |
| Filter plugin | Rust → `wasm32-unknown-unknown` |
| Dashboard | Next.js, Tailwind CSS |

---

## How the Plugin System Works

### Memory boundary

Go and Rust cannot share memory directly. The host (Go) and the plugin (Rust) communicate through the Wasm **linear memory** model:

1. Go serialises the HTTP request headers to JSON
2. Go calls `allocate_memory(n)` — Rust allocates a buffer and returns a pointer into its own linear memory
3. Go writes the JSON bytes into that address via Wazero's memory API
4. Go calls `process_request(ptr, len)` — Rust reads the slice, deserialises it, evaluates the filter rule, and returns `0` (allow) or `1` (block)
5. Go calls `free_memory(ptr, len, cap)` — Rust reconstructs and drops the `Vec`, releasing the allocation

### Concurrency model

Go's HTTP server handles every request on a separate goroutine. A single Wasm module instance has one linear memory space — concurrent writes would corrupt data. The solution is a **channel-based instance pool**:

- At startup, the Wasm module is **compiled once** (`wazero.Runtime.CompileModule`)
- **32 instances** are pre-instantiated from the compiled module, each with its own isolated memory, and placed into a buffered channel
- Each request goroutine **borrows** an instance from the channel, uses it, and **returns** it via `defer` — in the correct order, guaranteed by Go's LIFO defer stack
- If all instances are busy, the goroutine blocks on the channel until one becomes available

### Hot swap

The active pool is stored behind a `sync/atomic.Pointer[chan Instance]`. When a new `.wasm` is uploaded:

1. The new file is compiled into a fresh `CompiledModule`
2. A new pool of 32 instances is instantiated from it
3. `atomic.Pointer.Swap` atomically replaces the pointer — new requests start using the new pool immediately
4. In-flight requests hold a reference to the old channel and return their instance there; the old pool is garbage collected once the last borrow is returned

No locks. No request drops. No restart.

---

## Project Structure

```
gateway/
├── proxy/
│   └── cmd/
│       ├── main/
│       │   ├── main.go         # Runtime, pool init, server setup
│       │   ├── middleware.go   # Request pipeline, instance pool borrow/return
│       │   └── admin.go        # Upload, metrics, CORS handlers
│       └── backend/
│           └── backend.go      # Minimal upstream server for local testing
├── plugin/
│   └── filter/
│       └── src/lib.rs          # Rust filter plugin
└── dashboard/
    └── app/
        └── page.tsx            # Next.js control panel
```

---

## Getting Started

### Prerequisites

- Go 1.21+
- Rust with the `wasm32-unknown-unknown` target
- Node.js 18+

```bash
rustup target add wasm32-unknown-unknown
```

### 1. Build the Rust plugin

```bash
cd plugin/filter
cargo build --release --target wasm32-unknown-unknown
```

### 2. Start the upstream backend

```bash
cd proxy
go run ./cmd/backend/
```

### 3. Start the proxy

```bash
cd proxy
go run ./cmd/main/
```

The proxy listens on `:8080`. The admin API listens on `:8081`.

### 4. Start the dashboard

```bash
cd dashboard
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

---

## Usage

### Testing the filter

The default plugin blocks requests that carry a `Block: block` header:

```bash
# Allowed — proxied to the upstream
curl http://localhost:8080/

# Blocked — returns 403 Forbidden
curl http://localhost:8080/ -H "Block: block"
```

### Hot-swapping a plugin

1. Edit `plugin/filter/src/lib.rs` with new filter logic
2. Rebuild: `cargo build --release --target wasm32-unknown-unknown`
3. Open the dashboard at `localhost:3000`, select the new `.wasm` file, and click **Deploy Plugin**

The new filter is active immediately. The proxy never restarts.

### Live metrics

The dashboard polls `GET /admin/metrics` every two seconds and displays:

| Metric | Description |
|---|---|
| Total Requests | All requests processed since startup |
| Blocked Requests | Requests rejected by the active filter |
| Last Plugin Duration | Execution time of the most recent `process_request` call |

---

## Trade-offs

### Safety over raw speed

Wazero is a **pure-Go** Wasm runtime with no CGO dependency. This means the Rust plugin runs in a fully memory-safe sandbox — a buggy or malicious plugin cannot corrupt the host process. The trade-off is that a native FFI call would be faster. For a plugin system where plugins may be changed frequently and come from different authors, safety is the right default.

### Pool size is fixed

The instance pool is statically sized at 32. Under extreme concurrency (>32 simultaneous requests in the Wasm phase), excess goroutines block on the channel. A dynamic pool or per-core sizing would improve throughput at the cost of complexity.

### Shared linear memory per instance

Each pool instance has its own linear memory, so instances are independent. The instance struct bundles the module handle with its pre-looked-up function handles, avoiding repeated exports lookup on the hot path.
