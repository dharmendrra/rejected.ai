package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dharmendra/rejected.ai/internal/documents"
)

// llmTimeout bounds a request that makes one or more (possibly slow, local) LLM
// calls. Generous because gemma on Ollama can take many seconds per call.
const llmTimeout = 10 * time.Minute

// dbTimeout bounds a request that only touches MongoDB (no LLM), such as
// computing cross-interview trends.
const dbTimeout = 30 * time.Second

// maxUpload caps uploaded document size.
const maxUpload = 25 << 20 // 25 MiB

// readDocumentInput accepts either a multipart file upload (field "file") or a
// JSON body {"raw": "...", "filename": "..."} of pasted text, and returns the
// extracted plain text.
func readDocumentInput(r *http.Request) (string, error) {
	ct := r.Header.Get("Content-Type")
	if len(ct) >= 19 && ct[:19] == "multipart/form-data" {
		if err := r.ParseMultipartForm(maxUpload); err != nil {
			return "", fmt.Errorf("parse upload: %w", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return "", fmt.Errorf("read file field: %w", err)
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxUpload))
		if err != nil {
			return "", fmt.Errorf("read file: %w", err)
		}
		return documents.ExtractText(header.Filename, data)
	}

	var body struct {
		Raw      string `json:"raw"`
		Filename string `json:"filename"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxUpload)).Decode(&body); err != nil {
		return "", fmt.Errorf("decode body: %w", err)
	}
	if body.Filename != "" {
		return documents.ExtractText(body.Filename, []byte(body.Raw))
	}
	return body.Raw, nil
}

func (s *Server) handleIngestJD(w http.ResponseWriter, r *http.Request) {
	raw, err := readDocumentInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	jd, err := s.Documents.IngestJD(ctx, raw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, jd)
}

func (s *Server) handleIngestResume(w http.ResponseWriter, r *http.Request) {
	raw, err := readDocumentInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	cp, err := s.Documents.IngestResume(ctx, raw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cp)
}
