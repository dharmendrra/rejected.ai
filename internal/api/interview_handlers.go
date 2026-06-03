package api

import (
	"context"
	"encoding/json"
	"net/http"

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
	Interview  *domain.Interview           `json:"interview"`
	Graphs     *domain.CapabilityGraphSet  `json:"graphs"`
	Turns      []domain.Turn               `json:"turns"`
	Evidence   []domain.EvidenceItem       `json:"evidence"`
	Confidence []domain.ConfidenceSnapshot `json:"confidence"`
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

	var graphs domain.CapabilityGraphSet
	if err := s.Store.Coll(store.CollCapabilityGraphs).FindOne(ctx, bson.D{{Key: "interview_id", Value: id}}).Decode(&graphs); err == nil {
		view.Graphs = &graphs
	}

	view.Turns = findAll[domain.Turn](ctx, s, store.CollQuestions, id, "turn")
	view.Evidence = findAll[domain.EvidenceItem](ctx, s, store.CollEvidenceLedger, id, "turn")
	view.Confidence = findAll[domain.ConfidenceSnapshot](ctx, s, store.CollConfidenceScores, id, "turn")

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
