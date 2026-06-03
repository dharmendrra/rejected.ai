package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service computes and persists transcript signals for an interview.
type Service struct {
	Store       *store.Store
	Transcriber Transcriber // may be nil if not configured
}

// NewService constructs a media Service. transcriber may be nil.
func NewService(st *store.Store, t Transcriber) *Service {
	return &Service{Store: st, Transcriber: t}
}

// IngestTranscript computes measurable signals from a supplied transcript and
// persists them. This path always works (no transcription engine required).
func (s *Service) IngestTranscript(ctx context.Context, interviewID bson.ObjectID, turn int, text string, durationSec float64, latencyMs int, source string) (*domain.Transcript, error) {
	wordCount, wpm, fillerTotal, fillerRate, fillers := Analyze(text, durationSec)

	tr := domain.Transcript{
		InterviewID: interviewID,
		Turn:        turn,
		Source:      source,
		Text:        text,
		DurationSec: durationSec,
		WordCount:   wordCount,
		WPM:         wpm,
		FillerTotal: fillerTotal,
		FillerRate:  fillerRate,
		Fillers:     fillers,
		LatencyMs:   latencyMs,
		CreatedAt:   time.Now().UTC(),
	}

	// One transcript per (interview, turn): replace any prior.
	filter := bson.D{{Key: "interview_id", Value: interviewID}, {Key: "turn", Value: turn}}
	_, _ = s.Store.Coll(store.CollTranscripts).DeleteMany(ctx, filter)
	res, err := s.Store.Coll(store.CollTranscripts).InsertOne(ctx, tr)
	if err != nil {
		return nil, fmt.Errorf("persist transcript: %w", err)
	}
	tr.ID = res.InsertedID.(bson.ObjectID)
	return &tr, nil
}

// IngestAudio transcribes an audio file (if a transcriber is configured) and
// then computes + persists signals. durationSec/latencyMs are taken from the
// caller (whisper.cpp does not return duration without extra tooling).
func (s *Service) IngestAudio(ctx context.Context, interviewID bson.ObjectID, turn int, audioPath string, durationSec float64, latencyMs int) (*domain.Transcript, error) {
	if s.Transcriber == nil {
		return nil, fmt.Errorf("audio transcription not configured (set WHISPER_BIN + WHISPER_MODEL); supply a transcript via the transcript endpoint instead")
	}
	text, err := s.Transcriber.Transcribe(ctx, audioPath)
	if err != nil {
		return nil, err
	}
	return s.IngestTranscript(ctx, interviewID, turn, text, durationSec, latencyMs, domain.TranscriptWhisper)
}

// SaveTempAudio writes uploaded audio bytes to a temp file and returns the path
// and a cleanup func.
func SaveTempAudio(filename string, data []byte) (string, func(), error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".wav"
	}
	f, err := os.CreateTemp("", "rejected_ai_audio_*"+ext)
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", func() {}, err
	}
	f.Close()
	path := f.Name()
	cleanup := func() {
		os.Remove(path)
		os.Remove(path[:len(path)-len(ext)] + ".txt")
	}
	return path, cleanup, nil
}
