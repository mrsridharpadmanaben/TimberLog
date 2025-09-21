package api

import (
	"encoding/json"
	"net/http"

	"github.com/mrsridharpadmanaben/TimberLog/pkg/query"
)

type QueryServer struct {
	queryEngine *query.QueryEngine
}

func NewQueryServer(queryEngine *query.QueryEngine) *QueryServer {
	return &QueryServer{queryEngine: queryEngine}
}

// HTTP handler
func (qs *QueryServer) QueryHandler(w http.ResponseWriter, r *http.Request) {
	var query query.Query
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results, err := qs.queryEngine.Execute(&query)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Start server
func (qs *QueryServer) Start(addr string) error {
	http.HandleFunc("/query", qs.QueryHandler)

	return http.ListenAndServe(addr, nil)
}
