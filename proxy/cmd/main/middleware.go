package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/tetratelabs/wazero/api"
)

// Instance with function handles
type Instance struct {
	mod             api.Module
	allocate        api.Function
	process_request api.Function
	free_memory     api.Function
}

func middleware(ctx context.Context, pool chan Instance, proxy *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header_json, err := json.Marshal(r.Header)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("JSON marshal error: %v", err)
			return
		}

		// Borrow an instance
		instance := <-pool
		defer func() { pool <- instance }()

		// Allocate memory
		ptr, err := instance.allocate.Call(ctx, uint64(len(header_json)))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Wasm allocation error: %v", err)
			return
		}

		// Free memory afterwards
		defer func() {
			_, err := instance.free_memory.Call(ctx, ptr[0], uint64(len(header_json)), uint64(len(header_json)))
			if err != nil {
				log.Printf("Failed to free Wasm memory: %v", err)
			}
		}()

		// Write header
		if ok := instance.mod.Memory().Write(uint32(ptr[0]), header_json); !ok {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Check header
		res, err := instance.process_request.Call(ctx, ptr[0], uint64(len(header_json)))
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("Wasm execution error: %v", err)
			return
		}

		// Block if enabled
		if res[0] == 1 {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		proxy.ServeHTTP(w, r)
	}
}
