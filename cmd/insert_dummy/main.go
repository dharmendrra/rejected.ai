package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dharmendra/rejected.ai/internal/config"
	"github.com/dharmendra/rejected.ai/internal/domain"
	"github.com/dharmendra/rejected.ai/internal/store"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func main() {
	cfg, err := config.Load("/Users/dharmendra/golang-projects/rejected.ai/config.json")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	st, err := store.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("mongo: %v", err)
	}
	defer st.Disconnect(ctx)

	interviewID := bson.NewObjectID()
	jdID := bson.NewObjectID()
	candidateID := bson.NewObjectID()

	now := time.Now().UTC()

	// 1. Insert Job Description
	jd := domain.JobDescription{
		ID:                        jdID,
		Title:                     "Staff Software Engineer (Mock)",
		Raw:                       "Staff Software Engineer requirements: Distributed Systems, Go, Kafka, clickhouse, AWS.",
		Responsibilities:          []string{"Design large scale ingestion systems", "Lead technical architecture reviews", "Mentor senior engineers"},
		RequiredSkills:            []string{"Go", "Distributed Systems", "Kafka", "PostgreSQL"},
		PreferredSkills:           []string{"ClickHouse", "Kubernetes"},
		CreatedAt:                 now,
	}
	_, err = st.Coll(store.CollJobDescriptions).InsertOne(ctx, jd)
	if err != nil {
		log.Fatalf("insert jd: %v", err)
	}

	// 2. Insert Candidate Profile
	cp := domain.CandidateProfile{
		ID:           candidateID,
		Name:         "Alex Mercer (Mock)",
		Raw:          "Alex Mercer resume details.",
		Experience:   []string{"Lead Engineer at IngestionCorp (4 years)", "Senior Developer at MessageQueueInc (3 years)"},
		Technologies: []string{"Go", "Kafka", "AWS", "Redis"},
		CreatedAt:    now,
	}
	_, err = st.Coll(store.CollCandidateProfile).InsertOne(ctx, cp)
	if err != nil {
		log.Fatalf("insert cp: %v", err)
	}

	// 3. Insert Interview session
	iv := domain.Interview{
		ID:                 interviewID,
		JobDescriptionID:   jdID,
		CandidateProfileID: candidateID,
		Level:              "Staff Engineer",
		Type:               "System Design",
		DurationMin:        45,
		Status:             domain.StatusCompleted,
		Competencies:       []string{"System Architecture", "Scalability & Reliability", "Technical Leadership"},
		CreatedAt:          now.Add(-1 * time.Hour),
		UpdatedAt:          now,
	}
	_, err = st.Coll(store.CollInterviews).InsertOne(ctx, iv)
	if err != nil {
		log.Fatalf("insert interview: %v", err)
	}

	// 4. Insert Questions (Turns)
	turns := []domain.Turn{
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID,
			Turn:               1,
			Kind:               domain.TurnQuestion,
			Question:           "Explain a time when you had to design a highly scalable data ingestion system.",
			TargetCompetencies: []string{"System Architecture"},
			Answer:             "At IngestionCorp, I designed a distributed ingestion system processing 50k events per second using Kafka as a message broker, Go microservices for processing, and ClickHouse for analytical queries.",
			Answered:           true,
			CompressionRatio:   1.2,
			AskedAt:            now.Add(-50 * time.Minute),
			AnsweredAt:         now.Add(-48 * time.Minute),
			ResponseType:       domain.ResponseAnswer,
			ResponseReasoning:  "Candidate answered clearly with concrete details, scale numbers (50k/sec), and technical stack.",
		},
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID,
			Turn:               2,
			Kind:               domain.TurnQuestion,
			Question:           "How did you handle consistency issues and failures in that ingestion pipeline?",
			TargetCompetencies: []string{"Scalability & Reliability"},
			Answer:             "To handle consistency, we utilized idempotent consumers in Go by generating unique event IDs and tracking them in Redis. For pipeline failures, we implemented dead-letter queues and automated retries with exponential backoff.",
			Answered:           true,
			CompressionRatio:   1.3,
			AskedAt:            now.Add(-45 * time.Minute),
			AnsweredAt:         now.Add(-42 * time.Minute),
			ResponseType:       domain.ResponseAnswer,
			ResponseReasoning:  "Good answer detailing Redis caching for de-duplication, and dlq/retries strategy.",
		},
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID,
			Turn:               3,
			Kind:               domain.TurnQuestion,
			Question:           "How do you evaluate and prioritize technical debt when leading engineering teams?",
			TargetCompetencies: []string{"Technical Leadership"},
			Answer:             "I prioritize tech debt using a cost-value matrix and engineering alignment. We dedicate 20% of every sprint cycle to technical debt remediation, and we justify it by demonstrating the business impact or developer velocity improvement.",
			Answered:           true,
			CompressionRatio:   1.1,
			AskedAt:            now.Add(-40 * time.Minute),
			AnsweredAt:         now.Add(-37 * time.Minute),
			ResponseType:       domain.ResponseAnswer,
			ResponseReasoning:  "Understands the trade-offs of tech debt and shows a structured organizational process (20% sprint dedication).",
		},
	}
	for _, t := range turns {
		_, err = st.Coll(store.CollQuestions).InsertOne(ctx, t)
		if err != nil {
			log.Fatalf("insert turn: %v", err)
		}
	}

	// 5. Insert Evidence Items
	evidences := []domain.EvidenceItem{
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID,
			Turn:            1,
			Competency:      "System Architecture",
			Concepts:        []string{"ingestion", "kafka", "clickhouse", "go"},
			Polarity:        domain.PolarityPositive,
			Strength:        0.90,
			SupportingQuote: "designed a distributed ingestion system processing 50k events per second using Kafka",
			Interpretation:  "Candidate demonstrated real-world experience building high-throughput streaming systems.",
			CreatedAt:       now,
		},
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID,
			Turn:            2,
			Competency:      "Scalability & Reliability",
			Concepts:        []string{"idempotency", "redis", "dead-letter queue"},
			Polarity:        domain.PolarityPositive,
			Strength:        0.85,
			SupportingQuote: "idempotent consumers in Go by generating unique event IDs and tracking them in Redis",
			Interpretation:  "Understands distributed transaction/processing issues and correctly resolves consistency at the application layer.",
			CreatedAt:       now,
		},
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID,
			Turn:            3,
			Competency:      "Technical Leadership",
			Concepts:        []string{"prioritization", "tech debt", "alignment"},
			Polarity:        domain.PolarityPositive,
			Strength:        0.80,
			SupportingQuote: "dedicate 20% of every sprint cycle to technical debt remediation",
			Interpretation:  "Demonstrates strong organizational agility and capability to lead teams while managing code quality.",
			CreatedAt:       now,
		},
	}
	for _, evItem := range evidences {
		_, err = st.Coll(store.CollEvidenceLedger).InsertOne(ctx, evItem)
		if err != nil {
			log.Fatalf("insert evidence: %v", err)
		}
	}

	// 6. Insert Confidence Snapshots (Evolution timeline)
	snapshots := []domain.ConfidenceSnapshot{
		// Turn 1
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "System Architecture",
			Turn:          1,
			Confidence:    0.90,
			Cool:          0.95,
			Normal:        0.85,
			Hot:           0.70,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "High scoring introduction on Kafka and clickhouse design.",
			CreatedAt:     now,
		},
		// Turn 2
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "System Architecture",
			Turn:          2,
			Confidence:    0.90,
			Cool:          0.95,
			Normal:        0.85,
			Hot:           0.70,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "Unchanged from turn 1.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "Scalability & Reliability",
			Turn:          2,
			Confidence:    0.85,
			Cool:          0.90,
			Normal:        0.80,
			Hot:           0.65,
			EvidenceCount: 1,
			EvidenceTurns: []int{2},
			Rationale:     "Showed solid solution to reliability issues using DLQ and idempotency.",
			CreatedAt:     now,
		},
		// Turn 3
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "System Architecture",
			Turn:          3,
			Confidence:    0.90,
			Cool:          0.95,
			Normal:        0.85,
			Hot:           0.70,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "Unchanged.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "Scalability & Reliability",
			Turn:          3,
			Confidence:    0.85,
			Cool:          0.90,
			Normal:        0.80,
			Hot:           0.65,
			EvidenceCount: 1,
			EvidenceTurns: []int{2},
			Rationale:     "Unchanged.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "Technical Leadership",
			Turn:          3,
			Confidence:    0.80,
			Cool:          0.85,
			Normal:        0.75,
			Hot:           0.60,
			EvidenceCount: 1,
			EvidenceTurns: []int{3},
			Rationale:     "Described structured 20% team allocation process.",
			CreatedAt:     now,
		},
	}
	for _, snap := range snapshots {
		_, err = st.Coll(store.CollConfidenceScores).InsertOne(ctx, snap)
		if err != nil {
			log.Fatalf("insert snapshot: %v", err)
		}
	}

	// 7. Insert Competency Scores (Rollup)
	scores := []domain.CompetencyScore{
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "System Architecture",
			Confidence:    0.90,
			Cool:          0.95,
			Normal:        0.85,
			Hot:           0.70,
			EvidenceTurns: []int{1},
			Rationale:     "Clear expertise in distributed database scaling and stream processing.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "Scalability & Reliability",
			Confidence:    0.85,
			Cool:          0.90,
			Normal:        0.80,
			Hot:           0.65,
			EvidenceTurns: []int{2},
			Rationale:     "Understands common failure modes and correctly designs for idempotency and messaging safety.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID,
			Competency:    "Technical Leadership",
			Confidence:    0.80,
			Cool:          0.85,
			Normal:        0.75,
			Hot:           0.60,
			EvidenceTurns: []int{3},
			Rationale:     "Structured thinking around tech debt alignment and sprint velocity planning.",
			CreatedAt:     now,
		},
	}
	for _, sc := range scores {
		_, err = st.Coll(store.CollCompetencyScores).InsertOne(ctx, sc)
		if err != nil {
			log.Fatalf("insert score: %v", err)
		}
	}

	// 8. Insert Signals Doc
	sigs := domain.SignalsDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID,
		Signals: []domain.StrongestSignal{
			{
				Name:          "Distributed Stream Processing Experience",
				Description:   "Demonstrated deep execution skills with Apache Kafka, message partitioning, offsets, and consumer groups.",
				EvidenceTurns: []int{1, 2},
			},
			{
				Name:          "Sprint Tech Debt Integration",
				Description:   "Advocated a concrete 20% system for prioritizing refactoring tasks in alignment with product management.",
				EvidenceTurns: []int{3},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollSignals).InsertOne(ctx, sigs)
	if err != nil {
		log.Fatalf("insert signals: %v", err)
	}

	// 9. Insert Risks Doc
	risks := domain.RiskDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID,
		Risks: []domain.RiskItem{
			{
				Competency:    "Scalability & Reliability",
				Category:      domain.RiskJD,
				Severity:      "low",
				Reason:        "The candidate mentioned using Redis for idempotency but did not expand on cache eviction strategies or cache failure behaviors.",
				EvidenceTurns: []int{2},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollRiskAreas).InsertOne(ctx, risks)
	if err != nil {
		log.Fatalf("insert risks: %v", err)
	}

	// 10. Insert Recommendations
	rec := domain.Recommendation{
		ID:              bson.NewObjectID(),
		InterviewID:     interviewID,
		Decision:        domain.DecisionStrongHire,
		ConfidenceLevel: 0.88,
		Reasoning:       "Alex Mercer is a solid Staff Engineer candidate. They demonstrated concrete experience in system architecture, microservices, and queue management. The only minor gap is cache eviction handling, but this is a minor risk. Recommend strong hire.",
		Citations: []domain.Citation{
			{
				Competency: "System Architecture",
				Turns:      []int{1},
				Note:       "Handled 50k events per second successfully.",
			},
			{
				Competency: "Scalability & Reliability",
				Turns:      []int{2},
				Note:       "Detailed explanation of idempotency tracking using Redis.",
			},
		},
		Personas: []domain.PersonaView{
			{
				Persona:     "The Technical Architect",
				OverallTake: "Extremely strong on message queue patterns and storage isolation (ClickHouse). Highly endorsed.",
				Endorsements: []string{
					"Clear partition strategies",
					"Good usage of analytical OLAP store",
				},
				Concerns: []string{},
				PerCompetency: []domain.PersonaCompetency{
					{
						Competency: "System Architecture",
						Score:      0.95,
						Reasoning:  "Very clean design choices that isolate concerns.",
					},
				},
			},
			{
				Persona:     "The Practical Pragmatist",
				OverallTake: "A pragmatist who understands that delivery matters. Their tech debt prioritization framework was excellent.",
				Endorsements: []string{
					"Dedicating 20% sprint to debt is highly realistic",
				},
				Concerns: []string{
					"Should have explained Redis fallback or eviction mode",
				},
				PerCompetency: []domain.PersonaCompetency{
					{
						Competency: "Technical Leadership",
						Score:      0.85,
						Reasoning:  "Real-world understanding of developer velocity.",
					},
				},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollRecommendations).InsertOne(ctx, rec)
	if err != nil {
		log.Fatalf("insert recommendation: %v", err)
	}

	// 11. Insert Ideal Responses
	ideal := domain.IdealResponsesDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID,
		Responses: []domain.IdealResponse{
			{
				Question:     "Explain a time when you had to design a highly scalable data ingestion system.",
				Competency:   "System Architecture",
				KeyPoints:    []string{"backpressure controls", "horizontal partition scalability", "write micro-batching for analytical DBs"},
				SampleAnswer: "In highly scalable systems, it is critical to decoupled ingestion from execution using Kafka/Kinesis. For ClickHouse writes, you must micro-batch to prevent indexing overhead...",
			},
			{
				Question:     "How did you handle consistency issues and failures in that ingestion pipeline?",
				Competency:   "Scalability & Reliability",
				KeyPoints:    []string{"idempotent consumer keying", "DLQ processing flows", "circuit breaker patterns"},
				SampleAnswer: "Reliability requires end-to-end tracing. Use a hash of payload values as a transaction ID in Redis, set an expiration window, and route processing errors to DLQ.",
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollIdealResponses).InsertOne(ctx, ideal)
	if err != nil {
		log.Fatalf("insert ideal responses: %v", err)
	}

	// 12. Insert Report Progress (completed)
	steps := []domain.ReportStep{
		{Name: "Question 1: Evidence Extraction (LLM call 1/3)", Status: "completed"},
		{Name: "Question 1: Response Analysis (LLM call 2/3)", Status: "completed"},
		{Name: "Question 1: Confidence Rescoring (LLM call 3/3)", Status: "completed"},
		{Name: "Question 2: Evidence Extraction (LLM call 1/3)", Status: "completed"},
		{Name: "Question 2: Response Analysis (LLM call 2/3)", Status: "completed"},
		{Name: "Question 2: Confidence Rescoring (LLM call 3/3)", Status: "completed"},
		{Name: "Question 3: Evidence Extraction (LLM call 1/3)", Status: "completed"},
		{Name: "Question 3: Response Analysis (LLM call 2/3)", Status: "completed"},
		{Name: "Question 3: Confidence Rescoring (LLM call 3/3)", Status: "completed"},
		{Name: "Evaluator Personas", Status: "completed"},
		{Name: "Strongest Signals", Status: "completed"},
		{Name: "Risk Assessment", Status: "completed"},
		{Name: "Hiring Recommendation", Status: "completed"},
		{Name: "Ideal Response Guide", Status: "completed"},
	}
	progress := domain.ReportProgress{
		ID:             bson.NewObjectID(),
		InterviewID:    interviewID,
		Status:         "completed",
		TotalSteps:     len(steps),
		CompletedSteps: len(steps),
		CurrentStep:    "Completed",
		Steps:          steps,
		CreatedAt:      now.Add(-15 * time.Minute),
		UpdatedAt:      now,
	}
	_, err = st.Coll(store.CollReportProgress).InsertOne(ctx, progress)
	if err != nil {
		log.Fatalf("insert progress: %v", err)
	}

	// 13. Insert Capability Graph Set
	graphs := domain.CapabilityGraphSet{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID,
		Candidate: []domain.Capability{
			{Name: "Go", Category: "Programming", Evidence: []string{"Resume experience"}, Strength: 0.9},
			{Name: "Kafka", Category: "Messaging", Evidence: []string{"Ingestion design"}, Strength: 0.8},
		},
		Target: []domain.TargetCapability{
			{Name: "Go", Category: "Programming", Importance: "required", Weight: 0.8},
			{Name: "Kafka", Category: "Messaging", Importance: "required", Weight: 0.9},
		},
		Strengths: []string{"Hands-on Kafka experience", "Go microservices"},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollCapabilityGraphs).InsertOne(ctx, graphs)
	if err != nil {
		log.Fatalf("insert graphs: %v", err)
	}

	// ==========================================
	// SECOND MOCK ROUND: FAILED / LOW SCORES (<60% confidence)
	// ==========================================
	interviewID2 := bson.NewObjectID()
	jdID2 := bson.NewObjectID()
	candidateID2 := bson.NewObjectID()

	// 1. Job Description (Same role for direct comparison)
	jd2 := domain.JobDescription{
		ID:                        jdID2,
		Title:                     "Staff Software Engineer (Mock)",
		Raw:                       "Staff Software Engineer requirements: Distributed Systems, Go, Kafka, clickhouse, AWS.",
		Responsibilities:          []string{"Design large scale ingestion systems", "Lead technical architecture reviews", "Mentor senior engineers"},
		RequiredSkills:            []string{"Go", "Distributed Systems", "Kafka", "PostgreSQL"},
		PreferredSkills:           []string{"ClickHouse", "Kubernetes"},
		CreatedAt:                 now,
	}
	_, err = st.Coll(store.CollJobDescriptions).InsertOne(ctx, jd2)
	if err != nil {
		log.Fatalf("insert jd2: %v", err)
	}

	// 2. Candidate Profile
	cp2 := domain.CandidateProfile{
		ID:           candidateID2,
		Name:         "John Smith (Mock)",
		Raw:          "John Smith resume details.",
		Experience:   []string{"Senior Developer at LegacySystemsInc (5 years)"},
		Technologies: []string{"Java", "Go", "SQL"},
		CreatedAt:    now,
	}
	_, err = st.Coll(store.CollCandidateProfile).InsertOne(ctx, cp2)
	if err != nil {
		log.Fatalf("insert cp2: %v", err)
	}

	// 3. Interview session
	iv2 := domain.Interview{
		ID:                 interviewID2,
		JobDescriptionID:   jdID2,
		CandidateProfileID: candidateID2,
		Level:              "Staff Engineer",
		Type:               "System Design",
		DurationMin:        45,
		Status:             domain.StatusCompleted,
		Competencies:       []string{"System Architecture", "Scalability & Reliability", "Technical Leadership"},
		CreatedAt:          now.Add(-30 * time.Minute),
		UpdatedAt:          now,
	}
	_, err = st.Coll(store.CollInterviews).InsertOne(ctx, iv2)
	if err != nil {
		log.Fatalf("insert interview2: %v", err)
	}

	// 4. Questions (Turns)
	turns2 := []domain.Turn{
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID2,
			Turn:               1,
			Kind:               domain.TurnQuestion,
			Question:           "Explain a time when you had to design a highly scalable data ingestion system.",
			TargetCompetencies: []string{"System Architecture"},
			Answer:             "I worked on a system that had a lot of database writes. We just wrote directly to PostgreSQL. It worked fine for our load which wasn't huge, maybe a few hundred writes a minute.",
			Answered:           true,
			CompressionRatio:   1.0,
			AskedAt:            now.Add(-25 * time.Minute),
			AnsweredAt:         now.Add(-23 * time.Minute),
			ResponseType:       domain.ResponseAnswer,
			ResponseReasoning:  "Candidate answered with low technical depth, choosing basic postgres direct writes for a low-load scenario instead of standard streaming patterns.",
		},
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID2,
			Turn:               2,
			Kind:               domain.TurnQuestion,
			Question:           "How did you handle consistency issues and failures in that ingestion pipeline?",
			TargetCompetencies: []string{"Scalability & Reliability"},
			Answer:             "We didn't have many failures. If it failed, the client just retried. We didn't do anything special for duplicates, since postgres primary keys usually reject duplicate inserts anyway.",
			Answered:           true,
			CompressionRatio:   0.9,
			AskedAt:            now.Add(-20 * time.Minute),
			AnsweredAt:         now.Add(-18 * time.Minute),
			ResponseType:       domain.ResponseDeflection,
			ResponseReasoning:  "Candidate deflected the failure/reliability design questions, relying solely on client retries and postgres constraints, failing to show messaging pipeline safety.",
		},
		{
			ID:                 bson.NewObjectID(),
			InterviewID:        interviewID2,
			Turn:               3,
			Kind:               domain.TurnQuestion,
			Question:           "How do you evaluate and prioritize technical debt when leading engineering teams?",
			TargetCompetencies: []string{"Technical Leadership"},
			Answer:             "Usually we just fix things when they break. We don't have a formal process for tech debt. If someone wants to refactor something, they just do it on Friday afternoon.",
			Answered:           true,
			CompressionRatio:   1.1,
			AskedAt:            now.Add(-15 * time.Minute),
			AnsweredAt:         now.Add(-12 * time.Minute),
			ResponseType:       domain.ResponseAnswer,
			ResponseReasoning:  "Candidate lacks senior/staff level leadership. Processes are ad-hoc ('do it on Friday') without prioritization frameworks.",
		},
	}
	for _, t := range turns2 {
		_, err = st.Coll(store.CollQuestions).InsertOne(ctx, t)
		if err != nil {
			log.Fatalf("insert turn2: %v", err)
		}
	}

	// 5. Evidence Items
	evidences2 := []domain.EvidenceItem{
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID2,
			Turn:            1,
			Competency:      "System Architecture",
			Concepts:        []string{"postgres"},
			Polarity:        domain.PolarityNegative,
			Strength:        0.45,
			SupportingQuote: "just wrote directly to PostgreSQL. It worked fine for our load which wasn't huge",
			Interpretation:  "Lacks distributed architecture skills. Postgres direct writes do not scale for high ingestion.",
			CreatedAt:       now,
		},
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID2,
			Turn:            2,
			Competency:      "Scalability & Reliability",
			Concepts:        []string{"retries"},
			Polarity:        domain.PolarityNegative,
			Strength:        0.35,
			SupportingQuote: "We didn't do anything special for duplicates",
			Interpretation:  "Lacks distributed reliability understanding. Unaware of idempotency patterns.",
			CreatedAt:       now,
		},
		{
			ID:              bson.NewObjectID(),
			InterviewID:     interviewID2,
			Turn:            3,
			Competency:      "Technical Leadership",
			Concepts:        []string{"tech debt"},
			Polarity:        domain.PolarityNegative,
			Strength:        0.40,
			SupportingQuote: "just do it on Friday afternoon",
			Interpretation:  "Ad-hoc processes. Lacks capacity to scale development teams or prioritize tech debt.",
			CreatedAt:       now,
		},
	}
	for _, evItem := range evidences2 {
		_, err = st.Coll(store.CollEvidenceLedger).InsertOne(ctx, evItem)
		if err != nil {
			log.Fatalf("insert evidence2: %v", err)
		}
	}

	// 6. Confidence Snapshots
	snapshots2 := []domain.ConfidenceSnapshot{
		// Turn 1
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "System Architecture",
			Turn:          1,
			Confidence:    0.45,
			Cool:          0.55,
			Normal:        0.40,
			Hot:           0.25,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "Low-depth response using direct DB writes for system architecture.",
			CreatedAt:     now,
		},
		// Turn 2
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "System Architecture",
			Turn:          2,
			Confidence:    0.45,
			Cool:          0.55,
			Normal:        0.40,
			Hot:           0.25,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "Unchanged.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "Scalability & Reliability",
			Turn:          2,
			Confidence:    0.35,
			Cool:          0.45,
			Normal:        0.30,
			Hot:           0.15,
			EvidenceCount: 1,
			EvidenceTurns: []int{2},
			Rationale:     "Failed to address consistency, idempotency, or failover pipeline design.",
			CreatedAt:     now,
		},
		// Turn 3
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "System Architecture",
			Turn:          3,
			Confidence:    0.45,
			Cool:          0.55,
			Normal:        0.40,
			Hot:           0.25,
			EvidenceCount: 1,
			EvidenceTurns: []int{1},
			Rationale:     "Unchanged.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "Scalability & Reliability",
			Turn:          3,
			Confidence:    0.35,
			Cool:          0.45,
			Normal:        0.30,
			Hot:           0.15,
			EvidenceCount: 1,
			EvidenceTurns: []int{2},
			Rationale:     "Unchanged.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "Technical Leadership",
			Turn:          3,
			Confidence:    0.50,
			Cool:          0.60,
			Normal:        0.45,
			Hot:           0.30,
			EvidenceCount: 1,
			EvidenceTurns: []int{3},
			Rationale:     "Ad-hoc debt prioritization. Defers to free Friday time rather than team structure.",
			CreatedAt:     now,
		},
	}
	for _, snap := range snapshots2 {
		_, err = st.Coll(store.CollConfidenceScores).InsertOne(ctx, snap)
		if err != nil {
			log.Fatalf("insert snapshot2: %v", err)
		}
	}

	// 7. Competency Scores
	scores2 := []domain.CompetencyScore{
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "System Architecture",
			Confidence:    0.45,
			Cool:          0.55,
			Normal:        0.40,
			Hot:           0.25,
			EvidenceTurns: []int{1},
			Rationale:     "Demonstrated basic single-database setup knowledge but lacks distributed system scaling concepts.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "Scalability & Reliability",
			Confidence:    0.35,
			Cool:          0.45,
			Normal:        0.30,
			Hot:           0.15,
			EvidenceTurns: []int{2},
			Rationale:     "Struggles with pipeline failure models, event safety, and idempotency.",
			CreatedAt:     now,
		},
		{
			ID:            bson.NewObjectID(),
			InterviewID:   interviewID2,
			Competency:    "Technical Leadership",
			Confidence:    0.50,
			Cool:          0.60,
			Normal:        0.45,
			Hot:           0.30,
			EvidenceTurns: []int{3},
			Rationale:     "Lacks structured management, debt planning, or coaching models expected of a Staff Engineer.",
			CreatedAt:     now,
		},
	}
	for _, sc := range scores2 {
		_, err = st.Coll(store.CollCompetencyScores).InsertOne(ctx, sc)
		if err != nil {
			log.Fatalf("insert score2: %v", err)
		}
	}

	// 8. Signals
	sigs2 := domain.SignalsDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID2,
		Signals: []domain.StrongestSignal{
			{
				Name:          "Relational Database Schema Setup",
				Description:   "Understands basic schema design and primary keys in relational DB (PostgreSQL).",
				EvidenceTurns: []int{1, 2},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollSignals).InsertOne(ctx, sigs2)
	if err != nil {
		log.Fatalf("insert signals2: %v", err)
	}

	// 9. Risks
	risks2 := domain.RiskDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID2,
		Risks: []domain.RiskItem{
			{
				Competency:    "Scalability & Reliability",
				Category:      domain.RiskMissing,
				Severity:      "high",
				Reason:        "Candidate completely missed standard duplicate checking or reliability patterns in pipelines.",
				EvidenceTurns: []int{2},
			},
			{
				Competency:    "Technical Leadership",
				Category:      domain.RiskWeak,
				Severity:      "medium",
				Reason:        "Lacks systematic technical debt governance or sprint integration frameworks.",
				EvidenceTurns: []int{3},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollRiskAreas).InsertOne(ctx, risks2)
	if err != nil {
		log.Fatalf("insert risks2: %v", err)
	}

	// 10. Recommendations
	rec2 := domain.Recommendation{
		ID:              bson.NewObjectID(),
		InterviewID:     interviewID2,
		Decision:        domain.DecisionNoHire,
		ConfidenceLevel: 0.38, // 38% confidence, rose/pink warning color!
		Reasoning:       "John Smith did not demonstrate Staff level depth in any competency. System architecture choices were extremely basic (single PostgreSQL database) and lacked scalability patterns. They deflected critical reliability questions and lacked leadership frameworks. Recommend No Hire.",
		Citations: []domain.Citation{
			{
				Competency: "System Architecture",
				Turns:      []int{1},
				Note:       "Relied on postgres direct writes for data ingestion.",
			},
			{
				Competency: "Scalability & Reliability",
				Turns:      []int{2},
				Note:       "Struggled with duplicate messages and consistency.",
			},
		},
		Personas: []domain.PersonaView{
			{
				Persona:     "The Technical Architect",
				OverallTake: "Not suitable. Lacks basic distributed safety patterns. Answers were junior-level.",
				Endorsements: []string{},
				Concerns: []string{
					"Lacks stream processing knowledge",
					"Relies on DB constraints instead of message safety",
				},
				PerCompetency: []domain.PersonaCompetency{
					{
						Competency: "System Architecture",
						Score:      0.40,
						Reasoning:  "Very simple architecture suggestions.",
					},
				},
			},
			{
				Persona:     "The Practical Pragmatist",
				OverallTake: "No formal debt prioritization. The candidate's workflows are ad-hoc and unscalable.",
				Endorsements: []string{},
				Concerns: []string{
					"No tech debt tracking",
				},
				PerCompetency: []domain.PersonaCompetency{
					{
						Competency: "Technical Leadership",
						Score:      0.45,
						Reasoning:  "Lacks structured delegation/debt processes.",
					},
				},
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollRecommendations).InsertOne(ctx, rec2)
	if err != nil {
		log.Fatalf("insert recommendation2: %v", err)
	}

	// 11. Ideal Responses
	ideal2 := domain.IdealResponsesDoc{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID2,
		Responses: []domain.IdealResponse{
			{
				Question:     "Explain a time when you had to design a highly scalable data ingestion system.",
				Competency:   "System Architecture",
				KeyPoints:    []string{"backpressure controls", "horizontal partition scalability", "write micro-batching for analytical DBs"},
				SampleAnswer: "In highly scalable systems, it is critical to decoupled ingestion from execution using Kafka/Kinesis...",
			},
		},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollIdealResponses).InsertOne(ctx, ideal2)
	if err != nil {
		log.Fatalf("insert ideal responses2: %v", err)
	}

	// 12. Report Progress
	progress2 := domain.ReportProgress{
		ID:             bson.NewObjectID(),
		InterviewID:    interviewID2,
		Status:         "completed",
		TotalSteps:     len(steps),
		CompletedSteps: len(steps),
		CurrentStep:    "Completed",
		Steps:          steps,
		CreatedAt:      now.Add(-15 * time.Minute),
		UpdatedAt:      now,
	}
	_, err = st.Coll(store.CollReportProgress).InsertOne(ctx, progress2)
	if err != nil {
		log.Fatalf("insert progress2: %v", err)
	}

	// 13. Insert Capability Graph Set
	graphs2 := domain.CapabilityGraphSet{
		ID:          bson.NewObjectID(),
		InterviewID: interviewID2,
		Candidate: []domain.Capability{
			{Name: "SQL", Category: "Databases", Evidence: []string{"Resume experience"}, Strength: 0.6},
		},
		Target: []domain.TargetCapability{
			{Name: "Go", Category: "Programming", Importance: "required", Weight: 0.8},
			{Name: "Kafka", Category: "Messaging", Importance: "required", Weight: 0.9},
		},
		Strengths: []string{"SQL Database setup"},
		CreatedAt: now,
	}
	_, err = st.Coll(store.CollCapabilityGraphs).InsertOne(ctx, graphs2)
	if err != nil {
		log.Fatalf("insert graphs2: %v", err)
	}

	fmt.Printf("\n========================================================\n")
	fmt.Printf("MOCK INTERVIEW ROUNDS GENERATED SUCCESSFULLY!\n")
	fmt.Printf("========================================================\n")
	fmt.Printf("1. Candidate: %s (Strong Hire - 88%%)\n", cp.Name)
	fmt.Printf("   - Interview Turn Details: http://localhost:3000/interview/%s\n", interviewID.Hex())
	fmt.Printf("   - Hiring Intelligence Report: http://localhost:3000/interview/%s/report\n", interviewID.Hex())
	fmt.Printf("\n2. Candidate: %s (No Hire - 38%% Warning Color!)\n", cp2.Name)
	fmt.Printf("   - Interview Turn Details: http://localhost:3000/interview/%s\n", interviewID2.Hex())
	fmt.Printf("   - Hiring Intelligence Report: http://localhost:3000/interview/%s/report\n", interviewID2.Hex())
	fmt.Printf("========================================================\n\n")
}
