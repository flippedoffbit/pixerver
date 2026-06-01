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

	resp := map[string]interface{}{"filename": finalName, "path": finalPath}
	if token, ok, err := s.inputTokenFromRequest(r); err != nil {
		http.Error(w, "invalid input token", http.StatusBadRequest)
		logger.Warnf("postform: invalid input token: %v", err)
		return
	} else if ok {
		processor := s.Processor
		if processor.OutputDir == "" {
			processor.OutputDir = s.processedDir()
		}
		if processor.HTTPClient == nil {
			processor.HTTPClient = s.HTTPClient
		}
		result, err := processor.Process(r.Context(), token, finalPath)
		if err != nil {
			http.Error(w, "processing failed", http.StatusInternalServerError)
			logger.Errorf("postform: processing failed: %v", err)
			return
		}
		resp["processing"] = result
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
