package ledger

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
)

type AuditRecord struct {
	Timestamp        string `json:"timestamp"`
	RequestID        string `json:"request_id"`
	TeamID           string `json:"team_id"`
	SessionID        string `json:"session_id"`
	Model            string `json:"model"`
	PromptHash       string `json:"prompt_hash,omitempty"`
	StatusCode       int    `json:"status_code"`
	DurationMs       int64  `json:"duration_ms"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	MicroDollarCost  int64  `json:"cost_micro_usd"`
	CacheHit         bool   `json:"cache_hit"`
	BlockedReason    string `json:"blocked_reason,omitempty"`
}

type Ledger struct {
	channel chan AuditRecord
}

func NewLedger(filePath string) *Ledger {
	l := &Ledger{
		channel: make(chan AuditRecord, 10000), // Buffer to absorb spikes
	}

	// Start single worker for thread-safe sequential file I/O
	go l.worker(filePath)

	return l
}

func (l *Ledger) Record(record *AuditRecord) {
	select {
	case l.channel <- *record:
		// successfully queued
	default:
		log.Printf("Warning: Audit ledger channel full, dropping record %s", record.RequestID)
	}
}

func (l *Ledger) worker(filePath string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open audit log file: %v", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	for record := range l.channel {
		b, err := json.Marshal(record)
		if err != nil {
			log.Printf("Error marshalling audit record: %v", err)
			continue
		}

		_, err = writer.Write(append(b, '\n'))
		if err != nil {
			log.Printf("Error writing audit record to file: %v", err)
		}

		writer.Flush()
	}
}
