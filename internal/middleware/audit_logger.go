package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/finops-gateway/internal/ledger"
	"github.com/google/uuid"
)

const LogRecordKey string = "audit_record_ptr"

// AuditResponseRecorder captures the status code from inner middlewares
type AuditResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
}

func (rw *AuditResponseRecorder) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func AuditLogger(auditLedger *ledger.Ledger, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now().UTC()

		teamID := r.Header.Get("X-Team-ID")
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = uuid.New().String()
		}
		requestID := uuid.New().String()

		// 1. Initialize the mutable pointer
		record := &ledger.AuditRecord{
			Timestamp: startTime.Format(time.RFC3339),
			RequestID: requestID,
			TeamID:    teamID,
			SessionID: sessionID,
			// Other fields like Model, Cost, and PromptHash will be mutated by inner middlewares
		}

		// 2. Inject pointer into context
		ctx := context.WithValue(r.Context(), LogRecordKey, record)
		r = r.WithContext(ctx)

		// 3. Wrap response writer to capture final status code
		recorder := &AuditResponseRecorder{
			ResponseWriter: w,
			StatusCode:     http.StatusOK, // Default if never explicitly written
		}

		// 4. Execute the chain
		next.ServeHTTP(recorder, r)

		// 5. Post-Flight: Read the mutated pointer and capture the final status code and duration
		record.StatusCode = recorder.StatusCode
		record.DurationMs = time.Since(startTime).Milliseconds()
		
		// 6. Send to the asynchronous thread-safe JSONL ledger
		if auditLedger != nil {
			auditLedger.Record(record)
		}
	}
}
