package middleware

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/finops-gateway/internal/schema"
)

type contextKey string

const piiMapKey contextKey = "pii_map"

// PIIResponseRecorder intercepts the response for PII restoration
type PIIResponseRecorder struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	statusCode int
}

func (rw *PIIResponseRecorder) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *PIIResponseRecorder) Write(b []byte) (int, error) {
	return rw.Body.Write(b)
}

func PIIMasker(next http.HandlerFunc) http.HandlerFunc {
	// Simple regex patterns for PII
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	ssnPattern := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)

	return func(w http.ResponseWriter, r *http.Request) {
		piiMap := make(map[string]string)
		
		// 1. Read the injected struct pointer from Context
		isStream := false
		if payload, ok := r.Context().Value(StandardPayloadKey).(*schema.StandardPayload); ok {
			isStream = payload.Stream
			
			// Extract and mask Emails & SSNs in every message
			emailCounter := 1
			ssnCounter := 1

			for i, msg := range payload.Messages {
				bodyStr := msg.Content
				
				bodyStr = emailPattern.ReplaceAllStringFunc(bodyStr, func(match string) string {
					token := fmt.Sprintf("[EMAIL_%d]", emailCounter)
					piiMap[token] = match
					emailCounter++
					return token
				})
				
				bodyStr = ssnPattern.ReplaceAllStringFunc(bodyStr, func(match string) string {
					token := fmt.Sprintf("[SSN_%d]", ssnCounter)
					piiMap[token] = match
					ssnCounter++
					return token
				})

				// Mutate the struct directly in memory!
				payload.Messages[i].Content = bodyStr
			}
		}

		// If streaming, bypass response buffering so SSE chunks flush immediately to client
		if isStream {
			next.ServeHTTP(w, r)
			return
		}

		// Inject map into context so we can unmask the response
		ctx := context.WithValue(r.Context(), piiMapKey, piiMap)
		r = r.WithContext(ctx)

		// Create recorder to buffer upstream response
		recorder := &PIIResponseRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		// On return, retrieve map and unmask
		respStr := recorder.Body.String()
		for token, originalVal := range piiMap {
			respStr = strings.ReplaceAll(respStr, token, originalVal)
		}

		unmaskedBytes := []byte(respStr)
		w.Header().Set("Content-Length", strconv.Itoa(len(unmaskedBytes)))
		w.WriteHeader(recorder.statusCode)
		w.Write(unmaskedBytes)
	}
}
