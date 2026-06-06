package handlers

import (
	"encoding/json"
	"net/http"

	"pixerver/logger"
)

// PostFormHandler handles multipart file uploads from the form field "file".
// It stores the uploaded file under s.UploadDir with the filename pattern:
// <sha256>_<uuidv7>_<base32(originalName)>.extension
func (s *Server) PostFormHandler(w http.ResponseWriter, r *http.Request) {
	// limit request body size to 100MB to avoid OOM from huge uploads
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxUploadSize)

	if s.Validator == nil {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		logger.Errorf("postform: validator not configured")
		return
	}

	token, err := extractBearerToken(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("postform: %v", err)
		return
	}

	if _, err := s.Validator.Validate(token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		logger.Warnf("postform: jwt validation failed: %v", err)
		return
	}
	if s.Uploads == nil {
		http.Error(w, "upload queue unavailable", http.StatusServiceUnavailable)
		logger.Errorf("postform: upload queue unavailable")
		return
	}

	if err := r.ParseMultipartForm(s.MaxMemory); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		logger.Warnf("postform: parse multipart failed: %v", err)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		logger.Warnf("postform: FormFile error: %v", err)
		return
	}
	defer file.Close()

	finalName, finalPath, err := s.saveUploadedFile(file, header)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		logger.Errorf("postform: save upload failed: %v", err)
		return
	}

	logger.Infof("postform: stored upload as %s", finalPath)

	if token, ok, err := s.inputTokenFromRequest(r); err != nil {
		http.Error(w, "invalid input token", http.StatusBadRequest)
		logger.Warnf("postform: invalid input token: %v", err)
		return
	} else if ok {
		resp, err := s.Uploads.CreateUpload(r.Context(), finalName, finalPath, token)
		if err != nil {
			http.Error(w, "queue upload failed", http.StatusInternalServerError)
			logger.Errorf("postform: queue upload failed: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	http.Error(w, "input token required", http.StatusBadRequest)
	logger.Warnf("postform: input token missing")
}
