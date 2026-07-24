#!/usr/bin/env bash
# seed_demo.sh — end-to-end demo of the rejected.ai core engine.
#
# It ingests a sample JD + resume, starts an interview, and answers each
# generated question. The FIRST answer is deliberately terse shorthand
# ("duplicate handling"); a LATER answer fully explains idempotency/exactly-once
# and references the earlier shorthand. After the run, the script prints the
# confidence-evolution timeline and any RETROACTIVE evidence revisions — the
# defining behavior: earlier shorthand reinterpreted in light of later answers.
#
# Prereqs: server running on :8090, mongod up, ollama serving the configured model, jq.
set -euo pipefail

BASE="${BASE:-http://localhost:8090}"
HERE="$(cd "$(dirname "$0")" && pwd)"

say() { printf "\n\033[1;36m== %s ==\033[0m\n" "$1"; }

# Scripted answers. Answered in order against whatever question is asked.
# #1 is shorthand; #4 elaborates and ties back to #1.
ANSWERS=(
  "Mostly duplicate handling and retries."
  "I led a team of three and owned the payment ledger service end to end, including design reviews."
  "We sharded the ledger by tenant and used read replicas for heavy reconciliation queries."
  "For idempotency I derived a dedup key from the request, stored it in Redis with a TTL, and made payment capture exactly-once so gateway retries never double-charged. That's what I meant earlier by 'duplicate handling'."
  "I'd add circuit breakers and bulkheads, and use structured logging with trace IDs so on-call can triage incidents fast."
  "We tracked p99 latency against SLOs, rotated on-call weekly, and kept runbooks for each failure mode."
)

say "Health"
curl -sf "$BASE/healthz" | jq .

say "Ingest job description"
JD_RAW=$(jq -Rs . < "$HERE/sample_jd.txt")
JD_ID=$(curl -sf -X POST "$BASE/api/job-descriptions" \
  -H 'Content-Type: application/json' \
  -d "{\"raw\": $JD_RAW}" | jq -r .id)
echo "job_description_id=$JD_ID"

say "Ingest resume"
CV_RAW=$(jq -Rs . < "$HERE/sample_resume.txt")
CV_ID=$(curl -sf -X POST "$BASE/api/resumes" \
  -H 'Content-Type: application/json' \
  -d "{\"raw\": $CV_RAW}" | jq -r .id)
echo "candidate_profile_id=$CV_ID"

say "Create interview (Senior Engineer / Mixed / 20 min)"
CREATE=$(curl -sf -X POST "$BASE/api/interviews" \
  -H 'Content-Type: application/json' \
  -d "{\"job_description_id\":\"$JD_ID\",\"candidate_profile_id\":\"$CV_ID\",\"level\":\"Senior Engineer\",\"type\":\"Mixed\",\"duration_min\":20}")
IV_ID=$(echo "$CREATE" | jq -r .interview.id)
echo "interview_id=$IV_ID"
echo "Competencies: $(echo "$CREATE" | jq -c '.interview.competencies')"
echo "Q1: $(echo "$CREATE" | jq -r '.question.question')"

say "Answer loop"
for i in "${!ANSWERS[@]}"; do
  A="${ANSWERS[$i]}"
  echo "--- Answer $((i+1)): $A"
  RESP=$(curl -sf -X POST "$BASE/api/interviews/$IV_ID/answer" \
    -H 'Content-Type: application/json' \
    -d "$(jq -nc --arg a "$A" '{answer:$a}')")
  echo "    evidence extracted: $(echo "$RESP" | jq '.evidence | length')"
  COMPLETED=$(echo "$RESP" | jq -r .completed)
  if [ "$COMPLETED" = "true" ]; then
    echo "    interview completed."
    break
  fi
  echo "    next Q: $(echo "$RESP" | jq -r '.next.question // "(none)"')"
done

say "Confidence evolution (per competency, by turn)"
curl -sf "$BASE/api/interviews/$IV_ID" \
  | jq -r '.confidence | group_by(.competency)[] | "\(.[0].competency):\n" + (map("  turn \(.turn): normal=\(.normal) cool=\(.cool) hot=\(.hot)") | join("\n"))'

say "RETROACTIVE evidence revisions (proof: earlier evidence reinterpreted later)"
curl -sf "$BASE/api/interviews/$IV_ID" \
  | jq -r '[.evidence[] | select((.revisions // []) | length > 0)] as $r
           | if ($r | length) == 0
             then "  (no revisions emitted this run — re-run; depends on model output)"
             else ($r[] | "  [\(.competency)] turn \(.turn) quote=\(.supporting_quote|@json)\n" +
                   (.revisions[] | "    revised at turn \(.at_turn): \(.old_strength) -> \(.new_strength)  (\(.note))"))
             end'

say "Done"
echo "Full record: curl -s $BASE/api/interviews/$IV_ID | jq ."
