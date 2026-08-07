package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

type StreamRecorder struct {
	http.ResponseWriter
	cancel           context.CancelFunc
	lineBuffer       []byte
	wordBuffer       []string
	bannedPattern    *regexp.Regexp
	anomalyTriggered bool
}

func (sr *StreamRecorder) Write(b []byte) (int, error) {
	if sr.anomalyTriggered {
		// Drop the bytes if we've already triggered the guillotine
		return len(b), nil
	}

	for _, ch := range b {
		sr.lineBuffer = append(sr.lineBuffer, ch)
		if ch == '\n' {
			// Process full line
			line := string(sr.lineBuffer)
			
			if strings.HasPrefix(line, "data: ") && strings.TrimSpace(line) != "data: [DONE]" {
				payload := line[6:] // strip "data: "
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(payload), &data); err == nil {
					// Extract delta content
					if choices, ok := data["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok {
									// Maintain rolling buffer of 50 words
									words := strings.Fields(content)
									sr.wordBuffer = append(sr.wordBuffer, words...)
									if len(sr.wordBuffer) > 50 {
										sr.wordBuffer = sr.wordBuffer[len(sr.wordBuffer)-50:]
									}
									
									// 1. Regex Anomaly Detection
									recentText := strings.Join(sr.wordBuffer, " ")
									if sr.bannedPattern.MatchString(recentText) {
										sr.triggerGuillotine()
										return len(b), nil
									}

									// 2. Hallucination Loop Detection (10 words repeated 5 times)
									if checkHallucinationLoop(sr.wordBuffer) {
										sr.triggerGuillotine()
										return len(b), nil
									}
								}
							}
						}
					}
				}
			}
			
			// Write line to actual response
			sr.ResponseWriter.Write(sr.lineBuffer)
			if f, ok := sr.ResponseWriter.(http.Flusher); ok {
				f.Flush()
			}
			
			// Reset buffer for next line
			sr.lineBuffer = sr.lineBuffer[:0]
		}
	}
	return len(b), nil
}

func (sr *StreamRecorder) Flush() {
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

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

func (sr *StreamRecorder) triggerGuillotine() {
	sr.anomalyTriggered = true
	finalChunk := "data: {\"choices\": [{\"finish_reason\": \"content_filter\"}]}\n\n"
	sr.ResponseWriter.Write([]byte(finalChunk))
	if f, ok := sr.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
	// Call the context.CancelFunc to sever the underlying TCP connection to OpenAI
	sr.cancel()
}

func StreamInterceptor(next http.HandlerFunc) http.HandlerFunc {
	bannedPattern := regexp.MustCompile(`\[TOP_SECRET_PROJECT_X\]`)

	return func(w http.ResponseWriter, r *http.Request) {
		// Wrap the request context with a cancellable one
		ctx, cancel := context.WithCancel(r.Context())
		r = r.WithContext(ctx)

		recorder := &StreamRecorder{
			ResponseWriter: w,
			cancel:         cancel,
			lineBuffer:     make([]byte, 0),
			wordBuffer:     make([]string, 0),
			bannedPattern:  bannedPattern,
		}

		next.ServeHTTP(recorder, r)
		
		// Ensure context is cancelled eventually to prevent leaks
		cancel()
	}
}
