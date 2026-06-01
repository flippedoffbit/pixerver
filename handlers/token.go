package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"pixerver/models"
)

func (s *Server) inputTokenFromRequest(r *http.Request) (*models.InputToken, bool, error) {
	raw := strings.TrimSpace(r.FormValue("token"))
	if raw != "" {
		token, err := parseInputToken([]byte(raw))
		return token, true, err
	}
	if s.TokenPath == "" {
		return nil, false, nil
	}
	b, err := os.ReadFile(s.TokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	token, err := parseInputToken(b)
	return token, true, err
}

func parseInputToken(raw []byte) (*models.InputToken, error) {
	var token models.InputToken
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, err
	}
	if err := token.Validate(); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *Server) processedDir() string {
	if s.UploadDir == "" {
		return "processed"
	}
	return filepath.Join(s.UploadDir, "processed")
}
