package api

import (
	"encoding/json"
	"net/http"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/ingest"
	"github.com/mrsridharpadmanaben/TimberLog/pkg/types"
)

type WriteServer struct {
	ingestManager *ingest.IngestManager
}

func NewWriteServer(ingestManager *ingest.IngestManager) *WriteServer {
	return &WriteServer{ingestManager: ingestManager}
}

// HTTP handler
func (ws *WriteServer) WriteHandler(w http.ResponseWriter, r *http.Request) {
	var entry types.LogEntry

	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := ws.ingestManager.AppendLog(&entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func (ws *WriteServer) StopBackgroundFlush(w http.ResponseWriter, r *http.Request) {
	ws.ingestManager.StopBackgroundFlush()

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("ok"))
}

// Start server
func (ws *WriteServer) Start(addr string) error {

	http.HandleFunc("/write", ws.WriteHandler)
	http.HandleFunc("/stop", ws.StopBackgroundFlush)

	return http.ListenAndServe(addr, nil)
}
