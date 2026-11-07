package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// KomgaAuthRequired authenticates Komga-protocol clients.
//
// Komga clients (Mihon/Komga extension, Paperback, Panels, Chunky) support three
// credential styles and expect HTTP Basic to work, so this middleware accepts,
// in order:
//  1. an existing browser session or Bearer API key,
//  2. X-API-Key (Komga's native header) holding a Nowen API key,
//  3. HTTP Basic with the Nowen username + password, or username + API key.
//
// Basic auth is scoped to the Komga routes only so the plaintext password never
// becomes a general-purpose credential for the rest of the API.
func KomgaAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if user := GetCurrentUser(c); user != nil {
			c.Set(ContextKeyUser, user)
			c.Next()
			return
		}

		if headerKey := strings.TrimSpace(c.GetHeader("X-API-Key")); headerKey != "" {
			if key, user, err := store.AuthenticateAPIKey(headerKey); err == nil && key != nil && user != nil {
				setAuthenticatedUser(c, authUserFromModel(user), RequestCredential{Type: CredentialAPIKey, ID: key.ID})
				c.Next()
				return
			}
		}

		username, secret, ok := c.Request.BasicAuth()
		if ok && username != "" && secret != "" {
			// Username + API key (some clients put the key in the password field).
			if key, user, err := store.AuthenticateAPIKey(secret); err == nil && key != nil && user != nil && user.Username == username {
				setAuthenticatedUser(c, authUserFromModel(user), RequestCredential{Type: CredentialAPIKey, ID: key.ID})
				c.Next()
				return
			}
			// Username + password.
			user, err := store.GetUserByUsername(username)
			if err == nil && user != nil {
				if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(secret)) == nil {
					setAuthenticatedUser(c, authUserFromModel(user), RequestCredential{Type: CredentialSession, ID: "komga-basic"})
					c.Next()
					return
				}
			}
		}

		c.Header("WWW-Authenticate", `Basic realm="Nowen Reader", charset="UTF-8"`)
		c.Header("Cache-Control", "private, no-store")
		c.Header("Vary", "Authorization, Cookie, X-API-Key")
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	}
}
