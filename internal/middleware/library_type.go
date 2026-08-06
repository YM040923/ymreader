package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// LibraryTypeGuard restricts library create/update payloads to the two supported
// library types. Requests without a type field are left to the downstream
// handler, so partial updates and non-CRUD routes continue to work normally.
func LibraryTypeGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut {
			c.Next()
			return
		}
		if c.Request.Body == nil {
			c.Next()
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		if len(bytes.TrimSpace(body)) == 0 {
			c.Next()
			return
		}

		var payload struct {
			Type *string `json:"type"`
		}
		if err := json.Unmarshal(body, &payload); err != nil || payload.Type == nil {
			// Preserve the existing handler's validation and error response for
			// malformed JSON or create requests that omit required fields.
			c.Next()
			return
		}

		if *payload.Type != "comic" && *payload.Type != "novel" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Type must be comic or novel",
			})
			return
		}

		c.Next()
	}
}
