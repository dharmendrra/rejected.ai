package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// handleGenerateReport computes the final assessment (evaluator panel, signals,
// risk, recommendation). This makes several LLM calls and can be slow.
func (s *Server) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}

	// 1. Check if the report recommendation is already completed
	var rec domain.Recommendation
	if err := s.Store.Coll(store.CollRecommendations).FindOne(r.Context(), bson.D{{Key: "interview_id", Value: id}}).Decode(&rec); err == nil {
		rep, err := s.Report.Load(r.Context(), id)
		if err == nil {
			writeJSON(w, http.StatusOK, rep)
			return
		}
	}

	// 2. Check if a report progress already exists and is status "generating"
	var progress domain.ReportProgress
	if err := s.Store.Coll(store.CollReportProgress).FindOne(r.Context(), bson.D{{Key: "interview_id", Value: id}}).Decode(&progress); err == nil {
		if progress.Status == "generating" {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":   "generating",
				"progress": progress,
			})
			return
		}
	}

	// 3. Load answered turns to build the progress steps
	var turns []domain.Turn
	cur, err := s.Store.Coll(store.CollQuestions).Find(r.Context(), bson.D{{Key: "interview_id", Value: id}})
	if err == nil {
		_ = cur.All(r.Context(), &turns)
	}
	var answeredTurns []domain.Turn
	for _, t := range turns {
		if t.Answered {
			answeredTurns = append(answeredTurns, t)
		}
	}

	var steps []domain.ReportStep
	for _, t := range answeredTurns {
		steps = append(steps, domain.ReportStep{
			Name:   fmt.Sprintf("Question %d: Evidence Extraction (LLM call 1/3)", t.Turn),
			Status: "pending",
		})
		steps = append(steps, domain.ReportStep{
			Name:   fmt.Sprintf("Question %d: Response Analysis (LLM call 2/3)", t.Turn),
			Status: "pending",
		})
		steps = append(steps, domain.ReportStep{
			Name:   fmt.Sprintf("Question %d: Confidence Rescoring (LLM call 3/3)", t.Turn),
			Status: "pending",
		})
	}
	steps = append(steps, domain.ReportStep{Name: "Evaluator Personas", Status: "pending"})
	steps = append(steps, domain.ReportStep{Name: "Strongest Signals", Status: "pending"})
	steps = append(steps, domain.ReportStep{Name: "Risk Assessment", Status: "pending"})
	steps = append(steps, domain.ReportStep{Name: "Hiring Recommendation", Status: "pending"})
	steps = append(steps, domain.ReportStep{Name: "Ideal Response Guide", Status: "pending"})
	steps = append(steps, domain.ReportStep{Name: "Candidate Coaching Guide", Status: "pending"})

	progress = domain.ReportProgress{
		InterviewID:    id,
		Status:         "generating",
		TotalSteps:     len(steps),
		CompletedSteps: 0,
		CurrentStep:    steps[0].Name,
		Steps:          steps,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	// Delete any prior failed/completed progress docs
	_, _ = s.Store.Coll(store.CollReportProgress).DeleteMany(r.Context(), bson.D{{Key: "interview_id", Value: id}})
	_, err = s.Store.Coll(store.CollReportProgress).InsertOne(r.Context(), progress)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to initialize progress: "+err.Error())
		return
	}

	// 4. Start background goroutine to execute the steps
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Work on private copies so the HTTP response written below can read the
		// original progress/steps without racing this goroutine.
		steps := append([]domain.ReportStep(nil), steps...)
		progress := progress
		progress.Steps = steps

		setStepStatus := func(stepName string, status string) {
			foundCurrent := false
			for idx := range steps {
				if steps[idx].Name == stepName {
					steps[idx].Status = status
					if status == "running" {
						progress.CurrentStep = stepName
					}
					foundCurrent = true
				} else {
					if status == "running" {
						if steps[idx].Status == "running" {
							steps[idx].Status = "completed"
						} else if !foundCurrent && steps[idx].Status == "pending" {
							// Preceding turn that was skipped because it was already evaluated
							steps[idx].Status = "completed"
						}
					}
				}
			}
			s.Report.UpdateProgress(bgCtx, id, progress.CurrentStep, steps)
		}

		// Evaluate turns
		err := s.Interview.EvaluateAllTurns(bgCtx, id, func(turnNum int, subStep string) {
			var suffix string
			switch subStep {
			case "Evidence Extraction":
				suffix = "(LLM call 1/3)"
			case "Response Analysis":
				suffix = "(LLM call 2/3)"
			case "Confidence Rescoring":
				suffix = "(LLM call 3/3)"
			}
			setStepStatus(fmt.Sprintf("Question %d: %s %s", turnNum, subStep, suffix), "running")
		})
		if err != nil {
			s.Report.FailProgress(bgCtx, id, "evaluate answers: "+err.Error())
			return
		}

		// Evaluate report components
		_, err = s.Report.Generate(bgCtx, id, func(stepName string) {
			setStepStatus(stepName, "running")
		})
		if err != nil {
			s.Report.FailProgress(bgCtx, id, "generate report: "+err.Error())
			return
		}

		// Mark all steps as completed and finish
		for idx := range steps {
			steps[idx].Status = "completed"
		}
		s.Report.UpdateProgress(bgCtx, id, "Completed", steps)
		s.Report.CompleteProgress(bgCtx, id)
	}()

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "generating",
		"progress": progress,
	})
}

// handleGetReport returns a previously generated report from stored documents.
func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id, err := bson.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid interview id")
		return
	}
	rep, err := s.Report.Load(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if rep.Recommendation == nil && rep.Status != "generating" && rep.Status != "failed" {
		writeError(w, http.StatusNotFound, "report not generated yet; POST to this endpoint to generate")
		return
	}
	writeJSON(w, http.StatusOK, rep)
}
