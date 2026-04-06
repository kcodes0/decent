package node

import (
	"encoding/json"
	"net/http"
)

func (d *Daemon) publicHandler() http.Handler {
	root := d.serveRoot()
	return http.FileServer(http.Dir(root))
}

func (d *Daemon) adminHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", d.handleHealth)
	mux.HandleFunc("/status", d.handleStatus)
	mux.HandleFunc("/manifest", d.handleManifest)
	mux.HandleFunc("/sync", d.handleSync)
	return mux
}

func (d *Daemon) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := d.snapshot()
	if !status.Healthy {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"healthy": false,
			"error":   status.LastError,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"healthy": true})
}

func (d *Daemon) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, d.snapshot())
}

func (d *Daemon) handleManifest(w http.ResponseWriter, r *http.Request) {
	manifest, ok := d.manifestSnapshot()
	if !ok {
		http.Error(w, "manifest not loaded", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, manifest)
}

func (d *Daemon) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := d.syncOnce(r.Context()); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
