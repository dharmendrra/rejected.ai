package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type listQuestionsPondResponse struct {
	Questions []domain.PondQuestion `json:"questions"`
	Roles     []string              `json:"roles"`
	Types     []string              `json:"types"`
}

// handleListQuestionsPond handles GET /api/questions-pond.
// Supports optional filtering via ?role=... and ?type=... query params.
// Returns the matching list of questions sorted newest first, along with
// distinct role and type facets for frontend filter selection.
func (s *Server) handleListQuestionsPond(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := bson.D{}
	if role := strings.TrimSpace(r.URL.Query().Get("role")); role != "" && !strings.EqualFold(role, "all") {
		filter = append(filter, bson.E{Key: "role", Value: role})
	}
	if qType := strings.TrimSpace(r.URL.Query().Get("type")); qType != "" && !strings.EqualFold(qType, "all") {
		filter = append(filter, bson.E{Key: "type", Value: qType})
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(200)
	cur, err := s.Store.Coll(store.CollQuestionsPond).Find(ctx, filter, opts)
	if err != nil {
		log.Printf("[API] list questions pond: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to query questions pond: "+err.Error())
		return
	}

	var questions []domain.PondQuestion
	if err := cur.All(ctx, &questions); err != nil {
		log.Printf("[API] decode questions pond: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to decode questions pond: "+err.Error())
		return
	}
	if questions == nil {
		questions = []domain.PondQuestion{}
	}

	// Fetch dynamic facets
	roles := distinctStrings(ctx, s.Store.Coll(store.CollQuestionsPond), "role")
	types := distinctStrings(ctx, s.Store.Coll(store.CollQuestionsPond), "type")

	writeJSON(w, http.StatusOK, listQuestionsPondResponse{
		Questions: questions,
		Roles:     roles,
		Types:     types,
	})
}

// handleCountQuestionsPond handles GET /api/questions-pond/count.
// Returns the count of questions matching the given ?role=... and ?type=... filters.
func (s *Server) handleCountQuestionsPond(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := bson.D{}
	if role := strings.TrimSpace(r.URL.Query().Get("role")); role != "" && !strings.EqualFold(role, "all") {
		filter = append(filter, bson.E{Key: "role", Value: role})
	}
	if qType := strings.TrimSpace(r.URL.Query().Get("type")); qType != "" && !strings.EqualFold(qType, "all") {
		filter = append(filter, bson.E{Key: "type", Value: qType})
	}

	count, err := s.Store.Coll(store.CollQuestionsPond).CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("[API] count questions pond: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to count questions pond: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"count": count})
}

func distinctStrings(ctx context.Context, coll *mongo.Collection, field string) []string {
	var out []string
	res := coll.Distinct(ctx, field, bson.D{})
	if err := res.Decode(&out); err != nil {
		return []string{}
	}
	filtered := make([]string, 0, len(out))
	for _, s := range out {
		if strings.TrimSpace(s) != "" {
			filtered = append(filtered, s)
		}
	}
	return filtered
}
