package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/interview"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Server) handleCreateInterview(w http.ResponseWriter, r *http.Request) {
	var req interview.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	res, err := s.Interview.CreateSession(ctx, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) handleSubmitAnswer(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	var body struct {
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), llmTimeout)
	defer cancel()

	res, err := s.Interview.SubmitAnswer(ctx, id, body.Answer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// interviewView is the read model returned by GET /api/interviews/{id}.
type interviewView struct {
	Interview     *domain.Interview           `json:"interview"`
	Graphs        *domain.CapabilityGraphSet  `json:"graphs"`
	Turns         []domain.Turn               `json:"turns"`
	Evidence      []domain.EvidenceItem       `json:"evidence"`
	Confidence    []domain.ConfidenceSnapshot `json:"confidence"`
	Transcripts   []domain.Transcript         `json:"transcripts"`
	Video         []domain.VideoMetadata      `json:"video"`
	CandidateName string                      `json:"candidate_name"`
	JobTitle      string                      `json:"job_title"`
	JdRaw         string                      `json:"jd_raw"`
	CompletedAt   *time.Time                  `json:"completed_at,omitempty"`
}

func (s *Server) handleGetInterview(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	ctx := r.Context()
	view := interviewView{}

	var iv domain.Interview
	if err := s.Store.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&iv); err != nil {
		writeError(w, http.StatusNotFound, "interview not found")
		return
	}
	view.Interview = &iv

	var cp domain.CandidateProfile
	_ = s.Store.Coll(store.CollCandidateProfile).FindOne(ctx, bson.D{{Key: "_id", Value: iv.CandidateProfileID}}).Decode(&cp)
	view.CandidateName = cp.Name

	var jd domain.JobDescription
	_ = s.Store.Coll(store.CollJobDescriptions).FindOne(ctx, bson.D{{Key: "_id", Value: iv.JobDescriptionID}}).Decode(&jd)
	view.JobTitle = jd.Title
	view.JdRaw = jd.Raw

	var completedAt *time.Time
	var progress domain.ReportProgress
	if err := s.Store.Coll(store.CollReportProgress).FindOne(ctx, bson.D{{Key: "interview_id", Value: id}}).Decode(&progress); err == nil {
		if progress.Status == "completed" {
			completedAt = &progress.UpdatedAt
		}
	}
	if completedAt == nil {
		var rec domain.Recommendation
		if err := s.Store.Coll(store.CollRecommendations).FindOne(ctx, bson.D{{Key: "interview_id", Value: id}}).Decode(&rec); err == nil {
			completedAt = &rec.CreatedAt
		}
	}
	if completedAt == nil {
		completedAt = &iv.UpdatedAt
	}
	view.CompletedAt = completedAt

	var graphs domain.CapabilityGraphSet
	if err := s.Store.Coll(store.CollCapabilityGraphs).FindOne(ctx, bson.D{{Key: "interview_id", Value: id}}).Decode(&graphs); err == nil {
		view.Graphs = &graphs
	}

	view.Turns = findAll[domain.Turn](ctx, s, store.CollQuestions, id, "turn")
	view.Evidence = findAll[domain.EvidenceItem](ctx, s, store.CollEvidenceLedger, id, "turn")
	view.Confidence = findAll[domain.ConfidenceSnapshot](ctx, s, store.CollConfidenceScores, id, "turn")
	view.Transcripts = findAll[domain.Transcript](ctx, s, store.CollTranscripts, id, "turn")
	view.Video = findAll[domain.VideoMetadata](ctx, s, store.CollVideoMetadata, id, "turn")

	writeJSON(w, http.StatusOK, view)
}

// findAll loads all docs for an interview from a collection, sorted by sortKey.
func findAll[T any](ctx context.Context, s *Server, coll string, interviewID bson.ObjectID, sortKey string) []T {
	cur, err := s.Store.Coll(coll).Find(ctx,
		bson.D{{Key: "interview_id", Value: interviewID}},
		options.Find().SetSort(bson.D{{Key: sortKey, Value: 1}}),
	)
	if err != nil {
		return nil
	}
	var out []T
	_ = cur.All(ctx, &out)
	return out
}

func (s *Server) handleListInterviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cur, err := s.Store.Coll(store.CollInterviews).Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list interviews: "+err.Error())
		return
	}
	defer cur.Close(ctx)

	var list []map[string]any
	for cur.Next(ctx) {
		var iv domain.Interview
		if err := cur.Decode(&iv); err != nil {
			continue
		}

		var cp domain.CandidateProfile
		_ = s.Store.Coll(store.CollCandidateProfile).FindOne(ctx, bson.D{{Key: "_id", Value: iv.CandidateProfileID}}).Decode(&cp)

		var jd domain.JobDescription
		_ = s.Store.Coll(store.CollJobDescriptions).FindOne(ctx, bson.D{{Key: "_id", Value: iv.JobDescriptionID}}).Decode(&jd)

		var turns []domain.Turn
		curTurns, err := s.Store.Coll(store.CollQuestions).Find(ctx, bson.D{{Key: "interview_id", Value: iv.ID}}, options.Find().SetSort(bson.D{{Key: "turn", Value: 1}}))
		if err == nil {
			_ = curTurns.All(ctx, &turns)
		}

		var progress domain.ReportProgress
		var reportStatus string
		if err := s.Store.Coll(store.CollReportProgress).FindOne(ctx, bson.D{{Key: "interview_id", Value: iv.ID}}).Decode(&progress); err == nil {
			reportStatus = progress.Status
		}

		item := map[string]any{
			"id":                 iv.ID.Hex(),
			"level":              iv.Level,
			"type":               iv.Type,
			"status":             iv.Status,
			"report_status":      reportStatus,
			"created_at":         iv.CreatedAt,
			"updated_at":         iv.UpdatedAt,
			"candidate_name":     cp.Name,
			"resume_id":          cp.ID.Hex(),
			"resume_raw":         cp.Raw,
			"resume_tech":        cp.Technologies,
			"job_title":          jd.Title,
			"jd_id":              jd.ID.Hex(),
			"jd_raw":             jd.Raw,
			"questions":          turns,
		}
		list = append(list, item)
	}

	if list == nil {
		list = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleDeleteInterview(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	ctx := r.Context()

	// List of collections where we delete documents by interview_id
	collections := []string{
		store.CollQuestions,
		store.CollTranscripts,
		store.CollVideoMetadata,
		store.CollCapabilityGraphs,
		store.CollConfidenceScores,
		store.CollCompetencyScores,
		store.CollEvidenceLedger,
		store.CollSignals,
		store.CollRiskAreas,
		store.CollRecommendations,
		store.CollIdealResponses,
	}

	for _, coll := range collections {
		_, _ = s.Store.Coll(coll).DeleteMany(ctx, bson.D{{Key: "interview_id", Value: id}})
	}

	// Delete the interview session itself
	_, err = s.Store.Coll(store.CollInterviews).DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete interview: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
