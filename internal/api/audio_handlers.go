package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/media"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// handleIngestTranscript computes measurable signals from a supplied transcript.
// This path requires no transcription engine.
func (s *Server) handleIngestTranscript(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	var body struct {
		Turn        int     `json:"turn"`
		Transcript  string  `json:"transcript"`
		DurationSec float64 `json:"duration_sec"`
		LatencyMs   int     `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if body.Transcript == "" {
		writeError(w, http.StatusBadRequest, "transcript is required")
		return
	}
	tr, err := s.Media.IngestTranscript(r.Context(), id, body.Turn, body.Transcript, body.DurationSec, body.LatencyMs, domain.TranscriptProvided)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tr)
}

// handleIngestAudio transcribes an uploaded audio file (multipart field "file")
// then computes signals. Requires WHISPER_BIN + WHISPER_MODEL; otherwise returns
// 501 with guidance to use the transcript endpoint.
func (s *Server) handleIngestAudio(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeError(w, http.StatusBadRequest, "parse upload: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "read file field: "+err.Error())
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUpload))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read file: "+err.Error())
		return
	}

	turn, _ := strconv.Atoi(r.FormValue("turn"))
	durationSec, _ := strconv.ParseFloat(r.FormValue("duration_sec"), 64)
	latencyMs, _ := strconv.Atoi(r.FormValue("latency_ms"))

	path, cleanup, err := media.SaveTempAudio(header.Filename, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	tr, err := s.Media.IngestAudio(ctx, id, turn, path, durationSec, latencyMs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, tr)
}
