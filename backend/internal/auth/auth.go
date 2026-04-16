package auth

import (
	"fmt"
	"strings"
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
	Mode    string
	APIKey  string
}

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
