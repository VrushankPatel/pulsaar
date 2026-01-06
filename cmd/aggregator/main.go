package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type AuditLog struct {
	Timestamp string `json:"timestamp"`
	Operation string `json:"operation"`
	Path      string `json:"path"`
	AgentID   string `json:"agent_id,omitempty"`
}

var auditFile *os.File

func initAuditFile() error {
	auditLogPath := os.Getenv("PULSAAR_AUDIT_LOG_PATH")
	if auditLogPath == "" {
		auditLogPath = "/var/log/pulsaar/audit.log"
	}

	// Ensure directory exists
	dir := filepath.Dir(auditLogPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create audit log directory: %v", err)
	}

	var err error
	auditFile, err = os.OpenFile(auditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %v", err)
	}

	return nil
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

	// Write to file
	if auditFile != nil {
		if _, err := auditFile.WriteString(string(body) + "\n"); err != nil {
			log.Printf("Failed to write to audit log file: %v", err)
		}
		if err := auditFile.Sync(); err != nil {
			log.Printf("Failed to sync audit log file: %v", err)
		}
	}

	// Send to external system if configured
	if externalURL := os.Getenv("PULSAAR_EXTERNAL_LOG_URL"); externalURL != "" {
		resp, err := http.Post(externalURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Failed to send to external log: %v", err)
		} else {
			if err := resp.Body.Close(); err != nil {
				log.Printf("Error closing response body: %v", err)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, `{"status": "ok"}`); err != nil {
		log.Printf("Error writing health response: %v", err)
	}
}

func main() {
	if err := initAuditFile(); err != nil {
		log.Fatalf("Failed to initialize audit file: %v", err)
	}
	defer func() {
		if err := auditFile.Close(); err != nil {
			log.Printf("Error closing audit file: %v", err)
		}
	}()

	port := os.Getenv("PULSAAR_AGGREGATOR_PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/audit", handleAudit)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Audit aggregator listening on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
