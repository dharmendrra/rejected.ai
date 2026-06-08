package main

import (
	"context"
	"encoding/json"
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	st, err := store.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("mongo: %v", err)
	}
	defer st.Disconnect(ctx)

	targetID, err := bson.ObjectIDFromHex("6a2597f1fa3da6dd769345f6")
	if err != nil {
		log.Fatalf("invalid ID: %v", err)
	}

	// 1. Fetch Interview status
	var iv domain.Interview
	err = st.Coll(store.CollInterviews).FindOne(ctx, bson.D{{Key: "_id", Value: targetID}}).Decode(&iv)
	if err != nil {
		log.Fatalf("fetch interview: %v", err)
	}
	fmt.Printf("Interview Status: %s, Level: %s, Type: %s\n", iv.Status, iv.Level, iv.Type)

	// 2. Fetch Questions/Turns
	cur, err := st.Coll(store.CollQuestions).Find(ctx, bson.D{{Key: "interview_id", Value: targetID}})
	if err == nil {
		var turns []domain.Turn
		_ = cur.All(ctx, &turns)
		fmt.Printf("\n--- QUESTIONS / TURNS (%d total) ---\n", len(turns))
		for _, t := range turns {
			fmt.Printf("Turn %d | Answered: %t | ResponseType: %q | HasAssumptions: %t\n",
				t.Turn, t.Answered, t.ResponseType, len(t.Assumptions) > 0)
		}
	}

	// 3. Count Confidence Snapshots
	count, _ := st.Coll(store.CollConfidenceScores).CountDocuments(ctx, bson.D{{Key: "interview_id", Value: targetID}})
	fmt.Printf("\nConfidence Snapshots Count: %d\n", count)

	// 4. Count Evidence Ledger Items
	evidenceCount, _ := st.Coll(store.CollEvidenceLedger).CountDocuments(ctx, bson.D{{Key: "interview_id", Value: targetID}})
	fmt.Printf("Evidence Ledger Items: %d\n", evidenceCount)

	// 5. Query Report Progress
	var progress bson.M
	err = st.Coll(store.CollReportProgress).FindOne(ctx, bson.D{{Key: "interview_id", Value: targetID}}).Decode(&progress)
	if err != nil {
		fmt.Printf("Error fetching report progress: %v\n", err)
	} else {
		b, _ := json.MarshalIndent(progress, "", "  ")
		fmt.Printf("\nReport Progress JSON:\n%s\n", string(b))
	}

	// 6. Query Recommendations
	var rec domain.Recommendation
	err = st.Coll(store.CollRecommendations).FindOne(ctx, bson.D{{Key: "interview_id", Value: targetID}}).Decode(&rec)
	if err != nil {
		fmt.Printf("\nRecommendation: Not found (%v)\n", err)
	} else {
		fmt.Printf("\nRecommendation: Found (Decision: %s)\n", rec.Decision)
	}
}
