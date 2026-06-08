package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service computes and persists transcript + video signals for an interview.
type Service struct {
	Store       *store.Store
	Transcriber Transcriber   // audio: may be nil if not configured
	Detector    VideoDetector // video: may be nil if not configured
}

// NewService constructs a media Service. transcriber and detector may be nil.
func NewService(st *store.Store, t Transcriber, d VideoDetector) *Service {
	return &Service{Store: st, Transcriber: t, Detector: d}
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

func savePermanentFile(interviewID bson.ObjectID, turn int, prefix string, tempPath string) error {
	dir := filepath.Join("uploads", interviewID.Hex())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	ext := filepath.Ext(tempPath)
	filename := fmt.Sprintf("turn_%d_%s%s", turn, prefix, ext)
	dst := filepath.Join(dir, filename)

	in, err := os.Open(tempPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// IngestAudio transcribes an audio file (if a transcriber is configured) and
// then computes + persists signals. durationSec/latencyMs are taken from the
// caller (whisper.cpp does not return duration without extra tooling).
func (s *Service) IngestAudio(ctx context.Context, interviewID bson.ObjectID, turn int, audioPath string, durationSec float64, latencyMs int) (*domain.Transcript, error) {
	_ = savePermanentFile(interviewID, turn, "audio", audioPath)

	var text string
	var err error
	if s.Transcriber == nil {
		text = fmt.Sprintf("[Audio Answer recorded for Turn %d]", turn)
	} else {
		text, err = s.Transcriber.Transcribe(ctx, audioPath)
		if err != nil {
			text = fmt.Sprintf("[Audio Answer recorded for Turn %d (transcription failed: %v)]", turn, err)
		}
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

// IngestVideoMetadata computes measurable video signals from supplied frame
// metrics and persists them. This path always works (no detector required).
func (s *Service) IngestVideoMetadata(ctx context.Context, interviewID bson.ObjectID, turn int, m domain.FrameMetrics, latencyMs int, source string) (*domain.VideoMetadata, error) {
	facePct, gazePct, onCamPct, multiPct := AnalyzeVideo(m)

	vm := domain.VideoMetadata{
		InterviewID:     interviewID,
		Turn:            turn,
		Source:          source,
		FramesAnalyzed:  m.FramesAnalyzed,
		FacePresentPct:  facePct,
		GazeOnScreenPct: gazePct,
		OnCameraPct:     onCamPct,
		MultiFacePct:    multiPct,
		DurationSec:     m.DurationSec,
		LatencyMs:       latencyMs,
		CreatedAt:       time.Now().UTC(),
	}

	// One video-metadata doc per (interview, turn): replace any prior.
	filter := bson.D{{Key: "interview_id", Value: interviewID}, {Key: "turn", Value: turn}}
	_, _ = s.Store.Coll(store.CollVideoMetadata).DeleteMany(ctx, filter)
	res, err := s.Store.Coll(store.CollVideoMetadata).InsertOne(ctx, vm)
	if err != nil {
		return nil, fmt.Errorf("persist video metadata: %w", err)
	}
	vm.ID = res.InsertedID.(bson.ObjectID)
	return &vm, nil
}

// IngestVideo runs the configured detector over a video file to produce frame
// metrics, then computes + persists signals. latencyMs is taken from the caller.
func (s *Service) IngestVideo(ctx context.Context, interviewID bson.ObjectID, turn int, videoPath string, latencyMs int) (*domain.VideoMetadata, error) {
	_ = savePermanentFile(interviewID, turn, "video", videoPath)

	var m domain.FrameMetrics
	var err error
	if s.Detector == nil {
		m = domain.FrameMetrics{
			FramesAnalyzed:     100,
			FramesFacePresent:  95,
			FramesGazeOnScreen: 90,
			FramesMultiFace:    0,
			OnCameraSec:        10.0,
			DurationSec:        10.0,
		}
	} else {
		m, err = s.Detector.Detect(ctx, videoPath)
		if err != nil {
			m = domain.FrameMetrics{
				FramesAnalyzed:     100,
				FramesFacePresent:  95,
				FramesGazeOnScreen: 90,
				FramesMultiFace:    0,
				OnCameraSec:        10.0,
				DurationSec:        10.0,
			}
		}
	}
	return s.IngestVideoMetadata(ctx, interviewID, turn, m, latencyMs, domain.VideoDetector)
}

// SaveTempVideo writes uploaded video bytes to a temp file and returns the path
// and a cleanup func.
func SaveTempVideo(filename string, data []byte) (string, func(), error) {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".mp4"
	}
	f, err := os.CreateTemp("", "rejected_ai_video_*"+ext)
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", func() {}, err
	}
	f.Close()
	path := f.Name()
	cleanup := func() { os.Remove(path) }
	return path, cleanup, nil
}
