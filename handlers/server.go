package handlers

import (
	"net/http"

	"pixerver/internal/auth"
	"pixerver/pipeline"
)

// Server holds dependencies and configuration for HTTP handlers.
// Make dependencies explicit for easier testing and clarity.
type Server struct {
	Validator *auth.Validator
	// UploadDir is the directory where uploads are stored. Use an
	// absolute or relative path that your application controls.
	UploadDir string
	// MaxUploadSize caps the allowed request body size (bytes).
	MaxUploadSize int64
	// MaxMemory passed to ParseMultipartForm for in-memory part size.
	MaxMemory int64
	// TokenPath is used when an upload does not include a "token" form field.
	TokenPath string
	// Processor runs token-defined conversion work after an upload.
	Processor pipeline.Processor
	// HTTPClient is shared by pipeline backends and callbacks.
	HTTPClient *http.Client
}

// NewServer creates a Server with sane defaults. UploadDir may be
// overridden by tests or callers.
func NewServer(v *auth.Validator, uploadDir string) *Server {
	const (
		defaultMaxUpload = 100 << 20 // 100MB
		defaultMaxMemory = 32 << 20  // 32MB
	)
	return &Server{
		Validator:     v,
		UploadDir:     uploadDir,
		MaxUploadSize: defaultMaxUpload,
		MaxMemory:     defaultMaxMemory,
		Processor:     pipeline.Processor{},
	}
}
