package middleware

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/finops-gateway/internal/ledger"
)

// StreamRecorder wraps the ResponseWriter to scan SSE chunks in real-time
type StreamRecorder struct {
	http.ResponseWriter
	cancel           context.CancelFunc
	reqContext       context.Context // Need this to write to AuditRecord
	lineBuffer       []byte
	wordBuffer       []string
	bannedPattern    *regexp.Regexp
	anomalyTriggered bool
}

// Write intercepts the streaming bytes chunk by chunk
func (sr *StreamRecorder) Write(b []byte) (int, error) {
	if sr.anomalyTriggered {
		// Drop the bytes if the TCP socket is marked for termination
		return len(b), nil
	}

	for _, ch := range b {
		sr.lineBuffer = append(sr.lineBuffer, ch)
		if ch == '\n' {
			// A full line has been buffered
			line := string(sr.lineBuffer)
			
			// Parse OpenAI SSE delta format
			if strings.HasPrefix(line, "data: ") && strings.TrimSpace(line) != "data: [DONE]" {
				payload := line[6:] // strip "data: "
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &data); err == nil {
					if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok {
									
									// Maintain a rolling buffer of the last 50 words
									words := strings.Fields(content)
									sr.wordBuffer = append(sr.wordBuffer, words...)
									if len(sr.wordBuffer) > 50 {
										sr.wordBuffer = sr.wordBuffer[len(sr.wordBuffer)-50:]
									}
									
									recentText := strings.Join(sr.wordBuffer, " ")

									// 1. Data Leak Detection (Regex Anomaly)
									if sr.bannedPattern.MatchString(recentText) {
										log.Println("[Stream Guillotine] BLOCKED: Banned pattern detected in stream!")
										sr.triggerGuillotine("Data Leak Detected")
										return len(b), nil
									}

									// 2. Hallucination Loop Detection (Agent amnesia causing repetition)
									if checkHallucinationLoop(sr.wordBuffer) {
										log.Println("[Stream Guillotine] BLOCKED: Infinite hallucination loop detected!")
										sr.triggerGuillotine("Hallucination Loop")
										return len(b), nil
									}
								}
							}
						}
					}
				}
			}
			
			// Safe bytes: Write line to actual client and flush immediately
			sr.ResponseWriter.Write(sr.lineBuffer)
			if f, ok := sr.ResponseWriter.(http.Flusher); ok {
				f.Flush()
			}
			
			// Reset buffer for the next SSE line
			sr.lineBuffer = sr.lineBuffer[:0]
		}
	}
	return len(b), nil
}

// Flush ensures the wrapper implements http.Flusher
func (sr *StreamRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// checkHallucinationLoop checks if the last 10 words have been repeated 5 times in a row
func checkHallucinationLoop(words []string) bool {
	if len(words) < 50 {
		return false
	}
	last10 := strings.Join(words[len(words)-10:], " ")
	for i := 1; i < 5; i++ {
		block := strings.Join(words[len(words)-10*(i+1):len(words)-10*i], " ")
		if block != last10 {
			return false
		}
	}
	return true
}

// triggerGuillotine cancels the context, injecting a clean termination event to the client
func (sr *StreamRecorder) triggerGuillotine(reason string) {
	sr.anomalyTriggered = true
	
	// Inject a clean "content_filter" termination so the client UI doesn't hang
	finalChunk := "data: {\"choices\": [{\"finish_reason\": \"content_filter\"}]}\n\n"
	sr.ResponseWriter.Write([]byte(finalChunk))
	
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}

	if rec, ok := sr.reqContext.Value(LogRecordKey).(*ledger.AuditRecord); ok {
		rec.BlockedReason = "Streaming Guillotine: " + reason
		rec.StatusCode = 451 // Unavailable For Legal/Security Reasons
	}

	// Sever the underlying TCP connection to the upstream LLM immediately
	sr.cancel()
}

func StreamInterceptor(next http.HandlerFunc) http.HandlerFunc {
	// Example banned regex. In production, this would load from Tier-4 PostgreSQL config
	bannedPattern := regexp.MustCompile(`\[TOP_SECRET_PROJECT_X\]`)

	return func(w http.ResponseWriter, r *http.Request) {
		// Wrap the request context with a cancellable one so we can kill the TCP socket
		ctx, cancel := context.WithCancel(r.Context())
		r = r.WithContext(ctx)

		recorder := &StreamRecorder{
			ResponseWriter: w,
			cancel:         cancel,
			reqContext:     r.Context(),
			lineBuffer:     make([]byte, 0),
			wordBuffer:     make([]string, 0),
			bannedPattern:  bannedPattern,
		}

		next.ServeHTTP(recorder, r)
		
		// Ensure context is cancelled eventually to prevent leaks
		cancel()
	}
}
