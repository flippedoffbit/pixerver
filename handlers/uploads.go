package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"pixerver/logger"
	"pixerver/uploads"
)

func (s *Server) UploadStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Validator == nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		logger.Errorf("uploads: validator not configured")
		return
	}
	token, err := extractBearerToken(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("uploads: %v", err)
		return
	}
	if _, err := s.Validator.Validate(token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("uploads: jwt validation failed: %v", err)
		return
	}
	if s.Uploads == nil {
		http.Error(w, "upload store unavailable", http.StatusServiceUnavailable)
		return
	}
	uploadID := strings.TrimPrefix(r.URL.Path, "/uploads/")
	uploadID = strings.Trim(uploadID, "/")
	if uploadID == "" || strings.Contains(uploadID, "/") {
		http.NotFound(w, r)
		return
	}
	resp, err := s.Uploads.GetUploadStatus(r.Context(), uploadID)
	if err != nil {
		if errors.Is(err, uploads.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "server error", http.StatusInternalServerError)
		logger.Errorf("uploads: get status failed: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
