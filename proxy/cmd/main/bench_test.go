package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/tetratelabs/wazero"
)

const bench_wasm_path = "../../../plugin/filter/target/wasm32-unknown-unknown/release/filter.wasm"

// newBenchPool compiles the wasm module and returns a filled instance channel.
// The compiled module is shared across all instances, matching production behaviour.
func newBenchPool(ctx context.Context, b *testing.B, size int) (wazero.Runtime, chan Instance) {
	b.Helper()

	runtime := wazero.NewRuntime(ctx)
	b.Cleanup(func() { runtime.Close(ctx) })

	wasmBytes, err := os.ReadFile(bench_wasm_path)
	if err != nil {
		b.Fatalf("read wasm: %v", err)
	}

	compiledMod, err := runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		b.Fatalf("compile wasm: %v", err)
	}

	channel := make(chan Instance, size)
	for i := 0; i < size; i++ {
		mod, err := runtime.InstantiateModule(ctx, compiledMod, wazero.NewModuleConfig().WithName(""))
		if err != nil {
			b.Fatalf("instantiate module: %v", err)
		}

		allocate := mod.ExportedFunction("allocate_memory")
		process_request := mod.ExportedFunction("process_request")
		free_memory := mod.ExportedFunction("free_memory")

		if allocate == nil || process_request == nil || free_memory == nil {
			b.Fatal("missing exported function")
		}

		channel <- Instance{mod, allocate, process_request, free_memory}
	}

	return runtime, channel
}

// sampleHeaders returns JSON-encoded HTTP headers representative of a real request.
func sampleHeaders() []byte {
	headers := http.Header{
		"User-Agent":      {"Mozilla/5.0"},
		"Accept":          {"text/html,application/xhtml+xml"},
		"Accept-Language": {"en-US,en;q=0.9"},
		"Accept-Encoding": {"gzip, deflate, br"},
		"Connection":      {"keep-alive"},
	}
	data, _ := json.Marshal(headers)
	return data
}

// BenchmarkProcessRequest measures the full WASM pipeline for a single request:
// allocate → write → process_request → free. Runs sequentially.
func BenchmarkProcessRequest(b *testing.B) {
	ctx := context.Background()
	_, channel := newBenchPool(ctx, b, 1)

	header_json := sampleHeaders()
	instance := <-channel

	b.ResetTimer()
	for b.Loop() {
		ptr, err := instance.allocate.Call(ctx, uint64(len(header_json)))
		if err != nil {
			b.Fatal(err)
		}

		instance.mod.Memory().Write(uint32(ptr[0]), header_json)
		instance.process_request.Call(ctx, ptr[0], uint64(len(header_json)))
		instance.free_memory.Call(ctx, ptr[0], uint64(len(header_json)), uint64(len(header_json)))
	}
}

// BenchmarkProcessRequestParallel measures throughput when goroutines compete for
// pool instances — this is the production access pattern.
func BenchmarkProcessRequestParallel(b *testing.B) {
	ctx := context.Background()
	_, channel := newBenchPool(ctx, b, num_channel)

	header_json := sampleHeaders()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			instance := <-channel

			ptr, err := instance.allocate.Call(ctx, uint64(len(header_json)))
			if err != nil {
				channel <- instance
				b.Error(err)
				return
			}

			instance.mod.Memory().Write(uint32(ptr[0]), header_json)
			instance.process_request.Call(ctx, ptr[0], uint64(len(header_json)))
			instance.free_memory.Call(ctx, ptr[0], uint64(len(header_json)), uint64(len(header_json)))

			channel <- instance
		}
	})
}

// BenchmarkHeaderMarshal isolates the JSON marshalling cost so it can be
// subtracted from the full pipeline numbers above.
func BenchmarkHeaderMarshal(b *testing.B) {
	headers := http.Header{
		"User-Agent":      {"Mozilla/5.0"},
		"Accept":          {"text/html,application/xhtml+xml"},
		"Accept-Language": {"en-US,en;q=0.9"},
	}

	b.ResetTimer()
	for b.Loop() {
		json.Marshal(headers)
	}
}
