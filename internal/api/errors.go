package api

import "errors"

// Sentinel errors for health checks.
var (
	errDatabaseNil     = errors.New("database not initialized")
	errRouterNil       = errors.New("mikrotik router not initialized")
	errRouterUnreachable = errors.New("mikrotik router unreachable")
)

// Common response helpers will be added here.
