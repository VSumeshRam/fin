package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/finops-gateway/internal/adapter"
	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/middleware"
	"github.com/finops-gateway/internal/schema"
)

type StreamingResponseWriter struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	statusCode int
	headerSent bool
	// Fragmentation buffer for SSE
	fragBuffer []byte
	translator adapter.StreamTranslator
	isStream   bool
}

func (rw *StreamingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	if rw.isStream {
		rw.ResponseWriter.WriteHeader(code)
		rw.headerSent = true
	}
}

func (rw *StreamingResponseWriter) Write(b []byte) (int, error) {
	// If it's not a stream, buffer it in Body for ParseUsage & FormatResponse
	if !rw.isStream {
		return rw.Body.Write(b)
	}

	// It's a stream! Handle TCP fragmentation
	if !rw.headerSent {
		rw.ResponseWriter.WriteHeader(rw.statusCode)
		rw.headerSent = true
	}

	rw.fragBuffer = append(rw.fragBuffer, b...)
	
	// Process all complete chunks (delimited by \n\n or \r\n\r\n)
	var processedIdx int
	for {
		delimLen := 2
		idx := bytes.Index(rw.fragBuffer[processedIdx:], []byte("\n\n"))
		if idx == -1 {
			// Check for CRLF
			idx = bytes.Index(rw.fragBuffer[processedIdx:], []byte("\r\n\r\n"))
			if idx != -1 {
				delimLen = 4
			}
		}
		
		if idx == -1 {
			break // No more complete chunks
		}
		
		endIdx := processedIdx + idx + delimLen
		chunk := rw.fragBuffer[processedIdx:endIdx]
		
		// Sentinel Check
		if bytes.Contains(chunk, []byte("[DONE]")) {
			// Write unmodified sentinel
			rw.ResponseWriter.Write(chunk)
		} else if rw.translator != nil {
			_, translatedBytes, err := rw.translator.TranslateChunk(chunk)
			if err == nil {
				if len(translatedBytes) > 0 {
					rw.ResponseWriter.Write(translatedBytes)
				}
			} else {
				// Fallback if translation fails
				rw.ResponseWriter.Write(chunk)
			}
		} else {
			// No translator, pass through
			rw.ResponseWriter.Write(chunk)
		}
		
		// Flush immediately
		if f, ok := rw.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
		
		processedIdx = endIdx
	}
	
	// Retain only the un-processed fragments
	rw.fragBuffer = rw.fragBuffer[processedIdx:]
	
	// We always report we wrote the original bytes to satisfy the proxy io.Copy
	return len(b), nil
}

// Ensure it implements http.Flusher
func (rw *StreamingResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func NewHandler(upstreamURL string) (http.HandlerFunc, error) {
	target, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	
	// Universal Path Normalization:
	// Automatically adapts target paths for any upstream provider (OpenAI, Gemini, OpenRouter, Groq, Ollama)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		basePath := strings.TrimSuffix(target.Path, "/")
		if basePath != "" {
			req.URL.Path = basePath + "/chat/completions"
		} else {
			req.URL.Path = "/v1/chat/completions"
		}
	}

	openaiAdapter := &adapter.OpenAIAdapter{}
	anthropicAdapter := &adapter.AnthropicAdapter{}

	return func(w http.ResponseWriter, r *http.Request) {
		var promptHash string
		isStream := false
		var provider adapter.Provider = openaiAdapter
		
		// 1. Read the mutated struct pointer and select adapter
		if payloadPtr, ok := r.Context().Value(middleware.StandardPayloadKey).(*schema.StandardPayload); ok {
			isStream = payloadPtr.Stream

			// Dynamic Provider Selection based on requested model
			modelLower := strings.ToLower(payloadPtr.Model)
			if strings.Contains(modelLower, "claude") || strings.Contains(modelLower, "anthropic") {
				provider = anthropicAdapter
			}
			
			marshaledBytes, err := provider.FormatRequest(payloadPtr)
			if err == nil {
				// 2. Fix the Content-Length Mismatch trap!
				r.Body = io.NopCloser(bytes.NewBuffer(marshaledBytes))
				r.ContentLength = int64(len(marshaledBytes))
				r.Header.Set("Content-Length", strconv.Itoa(len(marshaledBytes)))
				
				hash := sha256.Sum256(marshaledBytes)
				promptHash = hex.EncodeToString(hash[:])
			}
		} else if r.Body != nil {
			// Fallback if inbound adapter failed or wasn't used
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			hash := sha256.Sum256(bodyBytes)
			promptHash = hex.EncodeToString(hash[:])
		}

		// Inject into AuditRecord if it exists in Context
		if rec, ok := r.Context().Value(middleware.LogRecordKey).(*ledger.AuditRecord); ok {
			rec.PromptHash = promptHash
		}

		var translator adapter.StreamTranslator
		if st, ok := provider.(adapter.StreamTranslator); ok {
			translator = st
		}

		wrapper := &StreamingResponseWriter{
			ResponseWriter: w,
			Body:           bytes.NewBuffer(nil),
			statusCode:     http.StatusOK,
			isStream:       isStream,
			translator:     translator,
		}

		// Prepare provider specific headers and target path
		provider.PrepareRequest(r)

		// Egress Secret Management: inject master API key if client omitted Authorization header
		if r.Header.Get("Authorization") == "" {
			if apiKey := os.Getenv("UPSTREAM_API_KEY"); apiKey != "" {
				r.Header.Set("Authorization", "Bearer "+apiKey)
			}
		}

		// Change host for reverse proxy to work with TLS correctly (e.g. api.openai.com)
		r.Host = target.Host

		proxy.ServeHTTP(wrapper, r)
		
		// Post-Flight: Parse Usage & Format Response for non-streaming responses
		if !isStream {
			respBytes := wrapper.Body.Bytes()
			if wrapper.statusCode == http.StatusOK && len(respBytes) > 0 {
				usage, err := provider.ParseUsage(respBytes)
				if err == nil && usage != nil {
					if ptr, ok := r.Context().Value(middleware.TokenUsageKey).(*schema.TokenUsage); ok {
						*ptr = *usage // Mutate the pointer synchronously
					}
				}

				if formatted, err := provider.FormatResponse(respBytes); err == nil {
					respBytes = formatted
				}
			}

			w.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(wrapper.statusCode)
			w.Write(respBytes)
		}
	}, nil
}
