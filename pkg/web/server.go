package web

import (
	"mime"
	"net/http"
)

// StaticHandler serves the browser frontend (index.html, wasm_exec.js,
// main.wasm). All speed measurements run client-side in the visitor's
// browser via the WASM engine, so the server only ships static assets.
func StaticHandler(dir string) http.Handler {
	mime.AddExtensionType(".wasm", "application/wasm")
	return http.FileServer(http.Dir(dir))
}
