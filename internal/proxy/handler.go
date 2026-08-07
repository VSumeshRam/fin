package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/finops-gateway/internal/ledger"
	"github.com/google/uuid"
)

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func NewHandler(upstreamURL string, auditLedger *ledger.Ledger) (http.HandlerFunc, error) {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = uuid.New().String()
		}
		requestID := uuid.New().String()

		var promptHash string
		if r.Body != nil {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			hash := sha256.Sum256(body)
			promptHash = hex.EncodeToString(hash[:])
		}

		wrapper := &responseWriterWrapper{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		// Change host for reverse proxy to work with TLS correctly (e.g. api.openai.com)
		r.Host = target.Host

		proxy.ServeHTTP(wrapper, r)

		// Record Audit asynchronously
		if auditLedger != nil {
			record := ledger.AuditRecord{
				RequestID:     requestID,
				TeamID:        teamID,
				SessionID:     sessionID,
				Model:         "default", // Extracted from payload in a real scenario
				PromptHash:    promptHash,
				StatusCode:    wrapper.statusCode,
				CostUSD:       0.01, // Example static cost for V1
				BlockedReason: nil,
			}
			auditLedger.Record(record)
		}
	}, nil
}
