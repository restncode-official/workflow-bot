package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"

	"github.com/pocketbase/pocketbase/core"
)

const (
	AdminUserEnv     = "WORKFLOW_ADMIN_USER"
	AdminPasswordEnv = "WORKFLOW_ADMIN_PASSWORD"
	defaultAdminUser = "admin"
)

func adminUser() string {
	if u := os.Getenv(AdminUserEnv); u != "" {
		return u
	}
	return defaultAdminUser
}

func adminPassword() string {
	return os.Getenv(AdminPasswordEnv)
}

// requireBasicAuth protects mutating admin routes with HTTP Basic Auth.
// Set WORKFLOW_ADMIN_PASSWORD (and optionally WORKFLOW_ADMIN_USER).
func requireBasicAuth(e *core.RequestEvent) error {
	password := adminPassword()
	if password == "" {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "admin password not configured (set WORKFLOW_ADMIN_PASSWORD)",
		})
	}

	user, pass, ok := e.Request.BasicAuth()
	if !ok || !secureEqual(user, adminUser()) || !secureEqual(pass, password) {
		e.Response.Header().Set("WWW-Authenticate", `Basic realm="workflow-admin"`)
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	return e.Next()
}

func secureEqual(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
