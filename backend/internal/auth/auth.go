package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

type Identity struct {
	Subject string
	Role    Role
	Mode    string // "apikey", "jwt", or "open"
	APIKey  string
}

// SynapseClaims is the JWT claims payload issued by Synapse.
type SynapseClaims struct {
	Subject string `json:"sub"`
	Role    Role   `json:"role"`
	jwt.RegisteredClaims
}

// IssueJWT creates a signed HS256 JWT for the given identity.
// secret must be a non-empty byte slice (from SYNAPSE_JWT_SECRET).
// The token expires in the given duration (use 0 for no expiry).
func IssueJWT(secret []byte, id *Identity, ttl time.Duration) (string, error) {
	if len(secret) == 0 {
		return "", fmt.Errorf("jwt secret is empty")
	}
	claims := SynapseClaims{
		Subject: id.Subject,
		Role:    id.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	if ttl != 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(ttl))
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(secret)
}

// ParseJWT verifies a signed JWT and returns the embedded Identity.
// Returns an error if the signature is invalid, the token is expired, or
// the embedded role is not one of the three valid values.
func ParseJWT(secret []byte, tokenStr string) (*Identity, error) {
	if len(secret) == 0 {
		return nil, fmt.Errorf("jwt secret is empty")
	}
	tok, err := jwt.ParseWithClaims(tokenStr, &SynapseClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid jwt: %w", err)
	}
	claims, ok := tok.Claims.(*SynapseClaims)
	if !ok || !tok.Valid {
		return nil, fmt.Errorf("invalid jwt claims")
	}
	if !IsValidRole(claims.Role) {
		return nil, fmt.Errorf("invalid role in jwt: %s", claims.Role)
	}
	return &Identity{Subject: claims.Subject, Role: claims.Role, Mode: "jwt"}, nil
}

// ParseToken parses a legacy "role:subject" plaintext bearer token.
// This is kept for backward compatibility in dev/open environments.
func ParseToken(token string) (*Identity, error) {
	parts := strings.Split(token, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	role := Role(strings.ToLower(strings.TrimSpace(parts[0])))
	subject := strings.TrimSpace(parts[1])
	if subject == "" {
		return nil, fmt.Errorf("missing subject")
	}
	if !IsValidRole(role) {
		return nil, fmt.Errorf("invalid role")
	}
	return &Identity{Subject: subject, Role: role, Mode: "jwt"}, nil
}

func IsValidRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

func Allows(role Role, required Role) bool {
	rank := map[Role]int{
		RoleViewer:   1,
		RoleOperator: 2,
		RoleAdmin:    3,
	}
	return rank[role] >= rank[required]
}
