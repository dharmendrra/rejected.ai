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

// handleIngestVideoMetadata computes measurable video signals from supplied
// per-frame metrics. This path requires no detection engine.
func (s *Server) handleIngestVideoMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	var body struct {
		Turn      int                 `json:"turn"`
		Metrics   domain.FrameMetrics `json:"metrics"`
		LatencyMs int                 `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if body.Metrics.FramesAnalyzed <= 0 && body.Metrics.DurationSec <= 0 {
		writeError(w, http.StatusBadRequest, "metrics.frames_analyzed or metrics.duration_sec is required")
		return
	}
	vm, err := s.Media.IngestVideoMetadata(r.Context(), id, body.Turn, body.Metrics, body.LatencyMs, domain.VideoProvided)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, vm)
}

// handleIngestVideo inspects an uploaded video file (multipart field "file") with
// the configured detector, then computes signals. Requires VIDEO_DETECTOR_BIN;
// otherwise returns 501 with guidance to use the video-metadata endpoint.
func (s *Server) handleIngestVideo(w http.ResponseWriter, r *http.Request) {
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
	latencyMs, _ := strconv.Atoi(r.FormValue("latency_ms"))

	path, cleanup, err := media.SaveTempVideo(header.Filename, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	vm, err := s.Media.IngestVideo(ctx, id, turn, path, latencyMs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, vm)
}
