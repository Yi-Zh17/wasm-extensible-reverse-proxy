package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/tetratelabs/wazero"
)

const (
	backend_url    = "http://localhost:9090"
	listen_on_port = ":8080"
	wasm_path      = "../plugin/filter/target/wasm32-unknown-unknown/release/filter.wasm"
)

func main() {
	// Parse upstream server url
	u, err := url.Parse(backend_url)
	if err != nil {
		log.Fatal(err)
	}

	// Read wasm
	file, err := os.ReadFile(wasm_path)
	if err != nil {
		log.Fatal(err)
	}

	// Initiate context
	ctx := context.Background()
	r := wazero.NewRuntime(ctx)
	defer r.Close(ctx)
	mod, _ := r.Instantiate(ctx, file)
	res, _ := mod.ExportedFunction("get_secret_number").Call(ctx)
	println(res[0])

	// Instantiate a reverse proxy engine
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(u)
			r.Out.Header.Set("Aero-proxy", "active")
			r.Out.Host = r.In.Host
		},
	}

	// Listen on port
	http.ListenAndServe(listen_on_port, proxy)
}
