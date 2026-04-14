# Aero Gateway

A programmable HTTP reverse proxy with a **WebAssembly plugin system**. Filter logic is written in Rust, compiled to `.wasm`, and hot-swapped at runtime via a web dashboard — no restarts required.

---

## Stack

| Layer | Technology |
|---|---|
| Proxy host | Go, `net/http/httputil` |
| Wasm runtime | [Wazero](https://github.com/tetratelabs/wazero) (pure-Go, no CGO) |
| Filter plugin | Rust → `wasm32-unknown-unknown` |
| Dashboard | Next.js, Tailwind CSS |

---

## Getting Started

**Prerequisites:** Go 1.21+, Rust, Node.js 18+

```bash
rustup target add wasm32-unknown-unknown
```

```bash
# 1. Build the plugin
cd plugin/filter
cargo build --release --target wasm32-unknown-unknown

# 2. Start the test backend (terminal 1)
cd proxy && go run ./cmd/backend/

# 3. Start the proxy (terminal 2)
cd proxy && go run ./cmd/main/

# 4. Start the dashboard (terminal 3)
cd dashboard && npm install && npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

---

## Usage

```bash
# Allowed — forwarded to upstream
curl http://localhost:8080/

# Blocked — 403 Forbidden
curl http://localhost:8080/ -H "Block: block"
```

To hot-swap the plugin: edit `plugin/filter/src/lib.rs`, rebuild, then upload the new `.wasm` via the dashboard. The new filter activates immediately.

---

## Benchmarks

Measured on an Intel i7-10875H. Run with `go test -bench=. -benchmem ./cmd/main/`.

| Benchmark | ns/op | What it measures |
|---|---|---|
| `ProcessRequest` | 7,671 ns | Full WASM pipeline, single goroutine |
| `ProcessRequestParallel` | 627 ns | Same pipeline, 16 goroutines, 32-instance pool |
| `HeaderMarshal` | 2,317 ns | JSON serialisation cost in isolation |

The parallel benchmark shows a ~12× throughput gain over sequential: the pool scales cleanly with goroutine count. JSON marshalling accounts for ~30% of single-threaded cost and is the main optimisation target if lower latency is needed.

---

## Trade-offs

**Safety over raw FFI speed.** Wazero runs the plugin in a memory-safe sandbox with no CGO — a buggy plugin cannot corrupt the host. A native FFI call would be faster but removes the isolation boundary.

**Fixed pool size.** The instance pool is statically sized at 32. Beyond 32 concurrent requests in the Wasm phase, goroutines block on the channel. A dynamic pool would improve peak throughput at the cost of complexity.

**JSON as the host–plugin protocol.** Passing headers as JSON is simple and correct but adds ~2.3 µs per request. A binary encoding (e.g. passing raw header bytes directly) would reduce this overhead.
