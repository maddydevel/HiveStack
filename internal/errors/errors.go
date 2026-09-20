// Copyright (c) 2026 Maddy AI Consultancy. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package errors provides a consistent error handling framework for HiveStack.
// Each error has a Code, Message, and can wrap an underlying error for
// structured error propagation across package boundaries.
package errors

import (
	"fmt"
)

// ErrorCode represents a machine-readable error code.
type ErrorCode string

const (
	// ErrNotFound indicates a requested resource was not found.
	ErrNotFound ErrorCode = "NOT_FOUND"
	// ErrUnauthorized indicates authentication is required or has failed.
	ErrUnauthorized ErrorCode = "UNAUTHORIZED"
	// ErrForbidden indicates the caller lacks permission for the operation.
	ErrForbidden ErrorCode = "FORBIDDEN"
	// ErrConflict indicates a resource conflict (e.g., duplicate name).
	ErrConflict ErrorCode = "CONFLICT"
	// ErrValidation indicates input validation failed.
	ErrValidation ErrorCode = "VALIDATION"
)

// Error is the standard error type for HiveStack. It carries a machine-readable
// code, a human-readable message, and optionally wraps an underlying error.
type Error struct {
	// Code is the machine-readable error code.
	Code ErrorCode
	// Message is a human-readable description of the error.
	Message string
	// Err is the underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for errors.Is / errors.As compatibility.
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new Error with the given code and message.
func New(code ErrorCode, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an existing error with a code and message.
func Wrap(code ErrorCode, message string, err error) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Is reports whether target has the same code as e.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// AsError attempts to convert an error to *Error. Returns nil if not an *Error.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if As(err, &e) {
		return e
	}
	return nil
}

// As is a thin wrapper around errors.As to avoid importing errors in callers.
func As(err error, target any) bool {
	return stdAs(err, target)
}

// stdAs delegates to errors.As via a helper to keep imports minimal.
func stdAs(err error, target any) bool {
	// This uses the standard library errors.As indirectly through a type assertion
	// chain. For full errors.As behavior, use errors.As directly.
	for err != nil {
		if matches(err, target) {
			return true
		}
		if unwrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = unwrapper.Unwrap()
		} else {
			return false
		}
	}
	return false
}

func matches(err error, target any) bool {
	switch t := target.(type) {
	case **Error:
		if e, ok := err.(*Error); ok {
			*t = e
			return true
		}
	}
	return false
}

// Sentinel errors for common cases. Use these with errors.Is for comparison.
var (
	// ErrNotFoundSentinel indicates a resource was not found.
	ErrNotFoundSentinel = New(ErrNotFound, "resource not found")
	// ErrUnauthorizedSentinel indicates authentication failure.
	ErrUnauthorizedSentinel = New(ErrUnauthorized, "authentication required")
	// ErrForbiddenSentinel indicates insufficient permissions.
	ErrForbiddenSentinel = New(ErrForbidden, "access denied")
	// ErrConflictSentinel indicates a resource conflict.
	ErrConflictSentinel = New(ErrConflict, "resource conflict")
	// ErrValidationSentinel indicates validation failure.
	ErrValidationSentinel = New(ErrValidation, "validation failed")
)

// NotFoundf creates a new NotFound error with a formatted message.
func NotFoundf(format string, args ...any) *Error {
	return New(ErrNotFound, fmt.Sprintf(format, args...))
}

// Unauthorizedf creates a new Unauthorized error with a formatted message.
func Unauthorizedf(format string, args ...any) *Error {
	return New(ErrUnauthorized, fmt.Sprintf(format, args...))
}

// Forbiddenf creates a new Forbidden error with a formatted message.
func Forbiddenf(format string, args ...any) *Error {
	return New(ErrForbidden, fmt.Sprintf(format, args...))
}

// Conflictf creates a new Conflict error with a formatted message.
func Conflictf(format string, args ...any) *Error {
	return New(ErrConflict, fmt.Sprintf(format, args...))
}

// Validationf creates a new Validation error with a formatted message.
func Validationf(format string, args ...any) *Error {
	return New(ErrValidation, fmt.Sprintf(format, args...))
}

// WrapNotFound wraps an error as NotFound.
func WrapNotFound(message string, err error) *Error {
	return Wrap(ErrNotFound, message, err)
}

// WrapUnauthorized wraps an error as Unauthorized.
func WrapUnauthorized(message string, err error) *Error {
	return Wrap(ErrUnauthorized, message, err)
}

// WrapForbidden wraps an error as Forbidden.
func WrapForbidden(message string, err error) *Error {
	return Wrap(ErrForbidden, message, err)
}

// WrapConflict wraps an error as Conflict.
func WrapConflict(message string, err error) *Error {
	return Wrap(ErrConflict, message, err)
}

// WrapValidation wraps an error as Validation.
func WrapValidation(message string, err error) *Error {
	return Wrap(ErrValidation, message, err)
}
