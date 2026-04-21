package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Trin9/SynapseFlow/backend/internal/audit"
	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/gin-gonic/gin"
)

const identityKey = "synapse_identity"

// authMiddleware validates the incoming request using either:
//   - Authorization: Bearer <role>:<subject>  (JWT-lite token mode)
//   - X-API-Key: <key>                        (API key mode, key looked up in configured set)
//
// If authentication passes, the Identity is stored in the Gin context.
// If AUTH_REQUIRED is false (no keys configured) the middleware is a no-op and
// injects a default admin identity so existing tests remain green.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		identity := s.resolveIdentity(c)
		if identity == nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
			c.Abort()
			return
		}
		c.Set(identityKey, identity)
		c.Next()
	}
}

// resolveIdentity extracts an Identity from the request without aborting.
// Returns nil if no valid credential was found and auth is configured.
// Returns a default admin identity when no API keys are configured (dev mode).
func (s *Server) resolveIdentity(c *gin.Context) *auth.Identity {
	// API key mode: X-API-Key header
	if key := strings.TrimSpace(c.GetHeader("X-API-Key")); key != "" {
		if id, ok := s.lookupAPIKey(key); ok {
			return id
		}
	}

	// Bearer token mode: Authorization: Bearer role:subject
	if bearer := strings.TrimSpace(c.GetHeader("Authorization")); bearer != "" {
		token := strings.TrimPrefix(bearer, "Bearer ")
		token = strings.TrimSpace(token)
		if token != "" {
			if id, err := auth.ParseToken(token); err == nil {
				return id
			}
		}
	}

	// No API keys configured → dev/open mode: inject anonymous admin so
	// the server is fully operational without credentials.
	if len(s.apiKeys) == 0 {
		return &auth.Identity{Subject: "anonymous", Role: auth.RoleAdmin, Mode: "open"}
	}
	return nil
}

// lookupAPIKey checks if key matches any configured API key and returns the associated Identity.
func (s *Server) lookupAPIKey(key string) (*auth.Identity, bool) {
	if id, ok := s.apiKeys[key]; ok {
		return id, true
	}
	return nil, false
}

// requireRole returns a middleware that enforces a minimum role.
// Must be used after authMiddleware (so identity is already in context).
func requireRole(required auth.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := identityFromCtx(c)
		if id == nil {
			writeError(c, http.StatusUnauthorized, "unauthorized", "authentication required", nil)
			c.Abort()
			return
		}
		if !auth.Allows(id.Role, required) {
			writeError(c, http.StatusForbidden, "forbidden", "insufficient role", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// identityFromCtx retrieves the Identity set by authMiddleware.
func identityFromCtx(c *gin.Context) *auth.Identity {
	v, ok := c.Get(identityKey)
	if !ok {
		return nil
	}
	id, _ := v.(*auth.Identity)
	return id
}

// auditMiddleware records sensitive operations after the request completes.
func (s *Server) auditMiddleware(action, resourceType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next() // execute handler first

		if s.audits == nil {
			return
		}
		id := identityFromCtx(c)
		actor := "anonymous"
		role := ""
		if id != nil {
			actor = id.Subject
			role = string(id.Role)
		}

		result := "success"
		if c.Writer.Status() >= 400 {
			result = "failure"
		}

		resourceID := c.Param("id")

		_ = s.audits.Record(context.Background(), audit.Entry{
			Actor:      actor,
			Role:       role,
			Action:     action,
			Resource:   resourceType,
			ResourceID: resourceID,
			Result:     result,
		})
	}
}
