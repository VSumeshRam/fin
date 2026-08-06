package ledger

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRecord struct {
	RequestID     string
	TeamID        string
	SessionID     string
	Model         string
	PromptHash    string
	StatusCode    int
	CostUSD       float64
	BlockedReason *string
}

type Ledger struct {
	pool    *pgxpool.Pool
	channel chan AuditRecord
}

func NewLedger(pool *pgxpool.Pool, workerCount int) *Ledger {
	l := &Ledger{
		pool:    pool,
		channel: make(chan AuditRecord, 10000), // Buffer to absorb spikes
	}

	// Start worker pool
	for i := 0; i < workerCount; i++ {
		go l.worker()
	}

	return l
}

func (l *Ledger) Record(record AuditRecord) {
	select {
	case l.channel <- record:
		// successfully queued
	default:
		log.Printf("Warning: Audit ledger channel full, dropping record %s", record.RequestID)
	}
}

func (l *Ledger) worker() {
	for record := range l.channel {
		err := l.insert(record)
		if err != nil {
			log.Printf("Error inserting audit log: %v", err)
		}
	}
}

func (l *Ledger) insert(record AuditRecord) error {
	query := `
		INSERT INTO audit_logs (
			request_id, team_id, session_id, model, prompt_hash,
			status_code, cost_usd, blocked_reason
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
	`
	_, err := l.pool.Exec(context.Background(), query,
		record.RequestID, record.TeamID, record.SessionID, record.Model, record.PromptHash,
		record.StatusCode, record.CostUSD, record.BlockedReason,
	)
	return err
}
