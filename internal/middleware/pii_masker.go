package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type contextKey string

const piiMapKey contextKey = "pii_map"

// ResponseRecorder intercepts the response for PII restoration
type ResponseRecorder struct {
	http.ResponseWriter
	Body *bytes.Buffer
}

func (rw *ResponseRecorder) Write(b []byte) (int, error) {
	return rw.Body.Write(b)
}

func PIIMasker(next http.HandlerFunc) http.HandlerFunc {
	// Simple regex patterns for PII
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	ssnPattern := regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)

	return func(w http.ResponseWriter, r *http.Request) {
		piiMap := make(map[string]string)
		
		if r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Failed to read body", http.StatusInternalServerError)
				return
			}
			
			// Try parsing as JSON to safely mutate "stream"
			var payload map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &payload); err == nil {
				// Mutate stream: true to stream: false
				if stream, ok := payload["stream"].(bool); ok && stream {
					payload["stream"] = false
				}
				
				// Re-marshal to string to apply regex masking easily, or marshal back directly
				newBodyBytes, _ := json.Marshal(payload)
				bodyStr := string(newBodyBytes)
				
				// Extract and mask Emails
				emailCounter := 1
				bodyStr = emailPattern.ReplaceAllStringFunc(bodyStr, func(match string) string {
					token := fmt.Sprintf("[EMAIL_%d]", emailCounter)
					piiMap[token] = match
					emailCounter++
					return token
				})
				
				// Extract and mask SSNs
				ssnCounter := 1
				bodyStr = ssnPattern.ReplaceAllStringFunc(bodyStr, func(match string) string {
					token := fmt.Sprintf("[SSN_%d]", ssnCounter)
					piiMap[token] = match
					ssnCounter++
					return token
				})

				maskedBytes := []byte(bodyStr)
				r.Body = io.NopCloser(bytes.NewBuffer(maskedBytes))
				r.ContentLength = int64(len(maskedBytes))
			} else {
				// If not valid JSON, just restore it unmodified
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Inject map into context
		ctx := context.WithValue(r.Context(), piiMapKey, piiMap)
		r = r.WithContext(ctx)

		// Create recorder to buffer upstream response
		recorder := &ResponseRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
		}

		next.ServeHTTP(recorder, r)

		// On return, retrieve map and unmask
		respStr := recorder.Body.String()
		for token, originalVal := range piiMap {
			respStr = strings.ReplaceAll(respStr, token, originalVal)
		}

		unmaskedBytes := []byte(respStr)
		w.Header().Set("Content-Length", strconv.Itoa(len(unmaskedBytes)))
		
		// Note: The status code and headers are already written by proxy, but we can write the body now
		w.Write(unmaskedBytes)
	}
}
