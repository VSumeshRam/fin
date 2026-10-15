package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/finops-gateway/internal/schema"
)

const StandardPayloadKey string = "standard_payload_ptr"

// InboundAdapter reads the raw request exactly once and injects the schema pointer
func InboundAdapter(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			next.ServeHTTP(w, r)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}

		// Restore body immediately in case middleware needs to read it directly
		// (though ideally, they should use the parsed struct pointer now)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		var payload schema.StandardPayload
		if err := json.Unmarshal(bodyBytes, &payload); err == nil {
			// Inject pointer into Context
			ctx := context.WithValue(r.Context(), StandardPayloadKey, &payload)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	}
}
