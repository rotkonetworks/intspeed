// Package endpoints holds the verified per-city test endpoints used by both
// the browser frontend and the CLI. Endpoints were probed for HTTPS
// reachability and CORS behavior; re-verify periodically as third-party
// servers come and go.
package endpoints

import (
	_ "embed"
	"encoding/json"
)

//go:embed endpoints.json
var rawEndpoints []byte

type Endpoint struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // ookla | file | librespeed | probe
	ID   string `json:"id,omitempty"`
	Host string `json:"host,omitempty"` // ookla host:port
	URL  string `json:"url,omitempty"`  // file/librespeed/probe base URL
	// Browser means the server sends permissive CORS headers, so a
	// cross-origin web page can read (and therefore time) its responses.
	Browser           bool   `json:"browser"`
	TimingAllowOrigin bool   `json:"timing_allow_origin,omitempty"`
	Upload            bool   `json:"upload"`
	ASN               string `json:"asn,omitempty"`
	ASName            string `json:"as_name,omitempty"` // short (<=10 chars)
	Note              string `json:"note,omitempty"`
}

// Raw returns the embedded registry JSON verbatim (for serving to the
// browser frontend).
func Raw() []byte { return rawEndpoints }

type LocationEndpoints struct {
	Name      string     `json:"name"`
	Endpoints []Endpoint `json:"endpoints"`
}

type Registry struct {
	Version   int                 `json:"version"`
	Verified  string              `json:"verified"`
	Locations []LocationEndpoints `json:"locations"`
}

func Load() (*Registry, error) {
	var r Registry
	if err := json.Unmarshal(rawEndpoints, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func (r *Registry) ForLocation(name string) *LocationEndpoints {
	for i := range r.Locations {
		if r.Locations[i].Name == name {
			return &r.Locations[i]
		}
	}
	return nil
}

// Browser returns only the endpoints usable directly from a web page.
func (l *LocationEndpoints) Browser() []Endpoint {
	var out []Endpoint
	for _, e := range l.Endpoints {
		if e.Browser {
			out = append(out, e)
		}
	}
	return out
}

// Ookla returns the endpoints testable by the CLI's speedtest engine.
func (l *LocationEndpoints) Ookla() []Endpoint {
	var out []Endpoint
	for _, e := range l.Endpoints {
		if e.Kind == "ookla" {
			out = append(out, e)
		}
	}
	return out
}
