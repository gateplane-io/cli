// Copyright (C) 2026 Ioannis Torakis <john.torakis@gmail.com>
// SPDX-License-Identifier: Elastic-2.0
//
// Licensed under the Elastic License 2.0.
// You may obtain a copy of the license at:
// https://www.elastic.co/licensing/elastic-license
//
// Use, modification, and redistribution permitted under the terms of the license,
// except for providing this software as a commercial service or product.

package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common cases - these can be checked with errors.Is()
var (
	ErrNoActiveRequest    = errors.New("no active request found")
	ErrExpiredGrant       = errors.New("grant code has expired")
	ErrGateNotFound       = errors.New("gate not found")
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrRequestNotFound    = errors.New("request not found")
	ErrInvalidGrantCode   = errors.New("invalid grant code")
	ErrAlreadyApproved    = errors.New("request already approved by current user")
	ErrInsufficientPerms  = errors.New("insufficient permissions")
	ErrVaultConnection    = errors.New("vault connection error")
	ErrInvalidGatePath    = errors.New("invalid gate path")
	ErrConfigurationError = errors.New("configuration error")
)

// VaultError provides structured error information for Vault operations
type VaultError struct {
	Operation string // The operation that failed (e.g., "create request", "approve request")
	Gate      string // The gate involved in the operation
	Err       error  // The underlying error
}

// Error implements the error interface
func (e *VaultError) Error() string {
	if e.Gate != "" {
		return fmt.Sprintf("failed to %s on gate %s: %v", e.Operation, e.Gate, e.Err)
	}
	return fmt.Sprintf("failed to %s: %v", e.Operation, e.Err)
}

// Unwrap returns the underlying error for error wrapping/unwrapping
func (e *VaultError) Unwrap() error {
	return e.Err
}

// Is checks if the error matches a target error (for sentinel error checking)
func (e *VaultError) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewVaultError creates a new VaultError with context
func NewVaultError(operation, gate string, err error) *VaultError {
	return &VaultError{
		Operation: operation,
		Gate:      gate,
		Err:       err,
	}
}

// NewVaultErrorf creates a new VaultError with formatted error message
func NewVaultErrorf(operation, gate, format string, args ...interface{}) *VaultError {
	return &VaultError{
		Operation: operation,
		Gate:      gate,
		Err:       fmt.Errorf(format, args...),
	}
}

// WrapVaultError wraps an error with Vault operation context
func WrapVaultError(operation, gate string, err error) error {
	if err == nil {
		return nil
	}

	// If it's already a VaultError, don't double-wrap
	var vaultErr *VaultError
	if errors.As(err, &vaultErr) {
		return err
	}

	return NewVaultError(operation, gate, err)
}
