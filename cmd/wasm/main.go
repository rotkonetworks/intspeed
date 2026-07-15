//go:build js && wasm

// The browser build of intspeed: exposes the portable engine to the page as
// window.intspeedStart / window.intspeedLocations. Network access goes
// through fetch, so only CORS-open registry endpoints are used.
package main

import (
	"context"
	"encoding/json"
	"syscall/js"

	"github.com/rotkonetworks/intspeed/pkg/endpoints"
	"github.com/rotkonetworks/intspeed/pkg/engine"
)

func main() {
	js.Global().Set("intspeedLocations", js.FuncOf(locationList))
	js.Global().Set("intspeedStart", js.FuncOf(start))
	select {}
}

// locationList() -> JSON array of location names, for pre-rendering the UI.
func locationList(js.Value, []js.Value) any {
	reg, err := endpoints.Load()
	if err != nil {
		return "[]"
	}
	names := make([]string, len(reg.Locations))
	for i, l := range reg.Locations {
		names[i] = l.Name
	}
	b, _ := json.Marshal(names)
	return string(b)
}

// start(callback, optsJSON) runs a sweep in a goroutine; every progress event
// and the final result set are delivered as JSON strings to callback.
func start(_ js.Value, args []js.Value) any {
	cb := args[0]
	opts := engine.Options{BrowserOnly: true}
	if len(args) > 1 && args[1].Type() == js.TypeString {
		var o struct {
			DownloadMB int64    `json:"downloadMB"`
			UploadMB   int64    `json:"uploadMB"`
			Locations  []string `json:"locations"`
		}
		if json.Unmarshal([]byte(args[1].String()), &o) == nil {
			opts.DownloadBytes = o.DownloadMB * 1_000_000
			opts.UploadBytes = o.UploadMB * 1_000_000
			opts.Locations = o.Locations
		}
	}

	go func() {
		reg, err := endpoints.Load()
		if err != nil {
			cb.Invoke(`{"type":"fatal","error":"registry load failed"}`)
			return
		}
		results := engine.Sweep(context.Background(), reg, opts, func(p engine.Progress) {
			b, _ := json.Marshal(p)
			cb.Invoke(string(b))
		})
		final, _ := json.Marshal(map[string]any{"type": "final", "results": results})
		cb.Invoke(string(final))
	}()
	return nil
}
