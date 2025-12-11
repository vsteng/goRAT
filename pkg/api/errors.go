package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a standard API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// SuccessResponse represents a standard API success response
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// RespondJSON writes a JSON response
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("ERROR: Failed to encode JSON response: %v", err)
	}
}

// RespondError writes an error response
func RespondError(w http.ResponseWriter, statusCode int, errorMsg string) {
	RespondJSON(w, statusCode, ErrorResponse{
		Error: errorMsg,
		Code:  statusCode,
	})
}

// RespondSuccess writes a success response
func RespondSuccess(w http.ResponseWriter, data interface{}, message string) {
	resp := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}
	RespondJSON(w, http.StatusOK, resp)
}

// GinRespondError responds with error in Gin context
func GinRespondError(c *gin.Context, statusCode int, errorMsg string) {
	c.JSON(statusCode, ErrorResponse{
		Error: errorMsg,
		Code:  statusCode,
	})
}

// GinRespondSuccess responds with success in Gin context
func GinRespondSuccess(c *gin.Context, data interface{}, message string) {
	resp := SuccessResponse{
		Success: true,
		Data:    data,
		Message: message,
	}
	c.JSON(http.StatusOK, resp)
}

// GinRespondJSON responds with JSON in Gin context
func GinRespondJSON(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, data)
}

// Common error messages
const (
	ErrInvalidRequest     = "invalid request"
	ErrUnauthorized       = "unauthorized"
	ErrForbidden          = "forbidden"
	ErrNotFound           = "not found"
	ErrInternalServer     = "internal server error"
	ErrInvalidCredentials = "invalid credentials"
	ErrSessionExpired     = "session expired"
	ErrUserNotFound       = "user not found"
	ErrClientNotFound     = "client not found"
	ErrProxyNotFound      = "proxy not found"
)
