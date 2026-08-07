package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
)

type PolicyEngine map[string][]string

func LoadPolicies(filepath string) (PolicyEngine, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var engine PolicyEngine
	if err := json.Unmarshal(bytes, &engine); err != nil {
		return nil, err
	}
	return engine, nil
}

func (pe PolicyEngine) IsAllowed(teamID, toolName string) bool {
	allowedTools, exists := pe[teamID]
	if !exists {
		return false
	}
	for _, t := range allowedTools {
		if t == "*" || t == toolName {
			return true
		}
	}
	return false
}

// MCPFirewall intercepts LLM tool_calls and enforces RBAC
func MCPFirewall(engine PolicyEngine, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")

		// Create a ResponseRecorder to buffer the upstream response
		recorder := &CacheRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK, // Default
		}

		next.ServeHTTP(recorder, r)

		// Inspect the JSON response payload for tool_calls
		respBytes := recorder.Body.Bytes()
		
		// Attempt to parse standard OpenAI completion format
		var payload map[string]interface{}
		unauthorized := false
		
		if err := json.Unmarshal(respBytes, &payload); err == nil {
			if choices, ok := payload["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if message, ok := choice["message"].(map[string]interface{}); ok {
						if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
							for _, tcRaw := range toolCalls {
								if tc, ok := tcRaw.(map[string]interface{}); ok {
									if function, ok := tc["function"].(map[string]interface{}); ok {
										if name, ok := function["name"].(string); ok {
											// Check RBAC
											if !engine.IsAllowed(teamID, name) {
												unauthorized = true
												break
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}

		if unauthorized {
			errorMsg := []byte(`{"error": "Unauthorized MCP tool execution blocked by Gateway."}`)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(errorMsg)))
			w.WriteHeader(http.StatusForbidden)
			w.Write(errorMsg)
			return
		}

		// Normal execution, pass through the buffer
		w.WriteHeader(recorder.statusCode)
		w.Write(respBytes)
	}
}
