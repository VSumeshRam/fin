package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/finops-gateway/internal/ledger"
)

// PolicyEngine maps Team IDs to a list of allowed MCP tool names
type PolicyEngine map[string][]string

// LoadPolicies reads the JSON RBAC matrix from disk
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

// IsAllowed checks if a specific team is authorized to execute a specific tool
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

// MCPFirewall intercepts LLM tool_calls (function execution payloads) and enforces RBAC
func MCPFirewall(engine PolicyEngine, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")

		// Buffer the upstream response so we can inspect it for tool_calls before sending it to the client
		recorder := &CacheRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK, // Default
		}

		// Execute downstream (which ultimately calls the Proxy)
		next.ServeHTTP(recorder, r)

		respBytes := recorder.Body.Bytes()
		
		// Attempt to parse standard OpenAI completion format to hunt for tool_calls
		var payload map[string]interface{}
		unauthorizedTool := ""
		
		if err := json.Unmarshal(respBytes, &payload); err == nil {
			if choices, ok := payload["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if message, ok := choice["message"].(map[string]interface{}); ok {
						if toolCalls, ok := message["tool_calls"].([]interface{}); ok {
							for _, tcRaw := range toolCalls {
								if tc, ok := tcRaw.(map[string]interface{}); ok {
									if function, ok := tc["function"].(map[string]interface{}); ok {
										if name, ok := function["name"].(string); ok {
											// Check RBAC Policy Matrix
											if !engine.IsAllowed(teamID, name) {
												unauthorizedTool = name
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

		// If a rogue tool call was detected, block the response physically
		if unauthorizedTool != "" {
			log.Printf("[MCP Firewall] BLOCKED: Team '%s' attempted unauthorized tool '%s'", teamID, unauthorizedTool)
			
			// Log the security incident in the Audit Ledger
			if rec, ok := r.Context().Value(LogRecordKey).(*ledger.AuditRecord); ok {
				rec.BlockedReason = "MCP Tool Forbidden: " + unauthorizedTool
				rec.StatusCode = http.StatusForbidden
			}

			errorMsg := []byte(`{"error": "Unauthorized MCP Tool Call: '` + unauthorizedTool + `' is forbidden for ` + teamID + `."}`)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(errorMsg)))
			w.WriteHeader(http.StatusForbidden)
			w.Write(errorMsg)
			return
		}

		// Safe execution: flush the buffered bytes to the client
		w.WriteHeader(recorder.statusCode)
		w.Write(respBytes)
	}
}
