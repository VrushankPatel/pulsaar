package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
)

type AuditLog struct {
	Timestamp string `json:"timestamp"`
	Operation string `json:"operation"`
	Path      string `json:"path"`
	AgentID   string `json:"agent_id,omitempty"`
}

func handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var audit AuditLog
	if err := json.Unmarshal(body, &audit); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log to stdout
	log.Printf("Received audit: %+v", audit)

	// Send to external system if configured
	if externalURL := os.Getenv("PULSAAR_EXTERNAL_LOG_URL"); externalURL != "" {
		resp, err := http.Post(externalURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to send to external log: %v", err)
		} else {
			resp.Body.Close()
		}
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	port := os.Getenv("PULSAAR_AGGREGATOR_PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/audit", handleAudit)

	log.Printf("Audit aggregator listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
