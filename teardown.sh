#!/usr/bin/env bash
#
# teardown.sh — undo what setup.sh did for rejected.ai.
#
#   Usage:   bash teardown.sh            # remove project data, model, artifacts; stop services
#            bash teardown.sh --purge    # ALSO uninstall the MongoDB & Ollama apps + delete config.json
#            bash teardown.sh -y         # skip the confirmation prompt
#
# By design it does NOT uninstall shared developer tools (Go, Node.js, Homebrew) —
# removing those would break your other projects. It prints how to remove them
# manually if you really want to.
#
# This is destructive. It asks for confirmation and lists exactly what it will do.

set -uo pipefail

ASSUME_YES=0
PURGE=0
for a in "$@"; do
  case "$a" in
    -y | --yes) ASSUME_YES=1 ;;
    --purge) PURGE=1 ;;
  esac
done

# ── pretty output (matches setup.sh) ─────────────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GRN=$'\033[32m'
  YLW=$'\033[33m'; CYN=$'\033[36m'; RST=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GRN=""; YLW=""; CYN=""; RST=""
fi
STEP=0; TOTAL=5
section() { STEP=$((STEP + 1)); printf "\n${BOLD}${CYN}━━ [%d/%d] %s ━━${RST}\n" "$STEP" "$TOTAL" "$1"; }
run()  { printf "   ${BOLD}▶${RST} %s\n" "$1"; }
ok()   { printf "   ${GRN}✓ %s${RST}\n" "$1"; }
warn() { printf "   ${YLW}! %s${RST}\n" "$1"; }
have() { command -v "$1" >/dev/null 2>&1; }

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -n "$REPO_DIR" ] && [ -f "$REPO_DIR/go.mod" ] || { printf "${RED}✗ cannot locate repo root${RST}\n"; exit 1; }
cd "$REPO_DIR"
OS="$(uname -s)"

# What setup created, read from config (fall back to defaults).
DB="$(grep -oE '"MONGO_DB"[[:space:]]*:[[:space:]]*"[^"]+"' config.json 2>/dev/null | sed -E 's/.*"([^"]+)"$/\1/')"
DB="${DB:-rejected_ai}"
MODEL="$(grep -oE '"OLLAMA_MODEL"[[:space:]]*:[[:space:]]*"[^"]+"' config.json 2>/dev/null | sed -E 's/.*"([^"]+)"$/\1/')"
MODEL="${MODEL:-gemma4:e4b}"

# Project-local paths that setup/builds create (always under REPO_DIR).
ARTIFACTS=(
  "bin"
  "web/node_modules"
  "web/.next"
  "web/out"
  "logs"
  "data/uploads"
  "data/media"
  "screenshots"
)

du_h() { [ -e "$1" ] && du -sh "$1" 2>/dev/null | awk '{print $1}' || echo "-"; }

# ── plan + confirm ───────────────────────────────────────────────────────────
printf "${BOLD}rejected.ai teardown${RST}  ${DIM}(%s)${RST}\n" "$OS"
printf "${DIM}Repo: %s${RST}\n" "$REPO_DIR"
printf "\n${BOLD}This will:${RST}\n"
printf "   • drop the MongoDB database ${BOLD}%s${RST} (project data)\n" "$DB"
printf "   • remove the Ollama model ${BOLD}%s${RST}\n" "$MODEL"
printf "   • stop the MongoDB and Ollama services started for this project\n"
printf "   • delete project artifacts:\n"
for p in "${ARTIFACTS[@]}"; do [ -e "$p" ] && printf "        - %-22s ${DIM}(%s)${RST}\n" "$p" "$(du_h "$p")"; done
if [ "$PURGE" -eq 1 ]; then
  printf "   ${RED}• --purge:${RST} uninstall the MongoDB & Ollama apps, and delete config.json (backed up to config.json.bak)\n"
fi
printf "\n${DIM}Will NOT remove: the repo source, Go, Node.js, or Homebrew (shared tools).${RST}\n"

printf "\n${BOLD}${RED}════════════════════════════════════════════════════════════════════${RST}\n"
printf "${BOLD}${RED} ⚠  WARNING — THIS PERMANENTLY DELETES YOUR INTERVIEW RECORDS${RST}\n"
printf "${BOLD}${RED}════════════════════════════════════════════════════════════════════${RST}\n"
printf "${RED} Every saved interview, report, evidence, score, and coaching note${RST}\n"
printf "${RED} in the '${BOLD}%s${RST}${RED}' database will be erased. ${BOLD}This CANNOT be undone.${RST}\n" "$DB"
printf "${RED} Back up your data first if you want to keep it.${RST}\n"

if [ "$ASSUME_YES" -ne 1 ]; then
  printf "\n${BOLD}${RED}Type 'yes' to permanently delete your interview records: ${RST}"
  REPLY=""
  got=1
  if read -r REPLY < /dev/tty 2>/dev/null; then
    got=0
  elif [ -t 0 ] && read -r REPLY; then
    got=0
  fi
  if [ "$got" -ne 0 ]; then
    printf "\n${YLW}! No interactive terminal detected, so I can't ask for confirmation.${RST}\n"
    printf "${DIM}  Re-run in a real terminal, or (only if you're sure) skip the prompt with:${RST} ${BOLD}./teardown.sh -y${RST}\n"
    exit 1
  fi
  case "$REPLY" in
    yes | YES) : ;;
    *) printf "${YLW}Aborted — nothing was removed.${RST}\n"; exit 0 ;;
  esac
fi

mongo_up() { (exec 3<>/dev/tcp/localhost/27017) 2>/dev/null && { exec 3>&- 3<&- 2>/dev/null; return 0; } || return 1; }
ollama_up() { (exec 3<>/dev/tcp/localhost/11434) 2>/dev/null && { exec 3>&- 3<&- 2>/dev/null; return 0; } || return 1; }

# ── [1/5] drop the database (while Mongo is still up) ─────────────────────────
section "MongoDB database ($DB)"
if mongo_up && have mongosh; then
  run "dropping database $DB"
  mongosh "mongodb://localhost:27017/$DB" --quiet --eval 'db.dropDatabase()' >/dev/null 2>&1 \
    && ok "dropped $DB" || warn "could not drop $DB (is mongosh working?)"
elif mongo_up; then
  warn "mongosh not found — skipping DB drop (install mongosh or drop $DB manually)"
else
  warn "MongoDB not running — skipping DB drop"
fi

# ── [2/5] remove the Ollama model (while Ollama is still up) ──────────────────
section "Ollama model ($MODEL)"
if have ollama; then
  run "removing model $MODEL"
  ollama rm "$MODEL" >/dev/null 2>&1 && ok "removed $MODEL" || warn "model $MODEL not present / already gone"
else
  warn "ollama not installed — skipping"
fi

# ── [3/5] stop services ──────────────────────────────────────────────────────
section "Services"
if [ "$OS" = "Darwin" ] && have brew; then
  brew services stop mongodb-community >/dev/null 2>&1 && ok "stopped mongodb-community" || warn "mongodb-community service not running"
  brew services stop ollama >/dev/null 2>&1 && ok "stopped ollama service" || true
fi
pkill -f "ollama serve" >/dev/null 2>&1 && ok "stopped ollama serve" || true
if have docker && docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q '^rejected-mongo$'; then
  run "removing Docker mongo container (rejected-mongo)"
  docker rm -f rejected-mongo >/dev/null 2>&1 && ok "removed container rejected-mongo" || true
fi
if [ "$OS" != "Darwin" ]; then
  sudo systemctl stop mongod 2>/dev/null || sudo systemctl stop mongodb 2>/dev/null || true
fi

# ── [4/5] delete project artifacts ───────────────────────────────────────────
section "Project artifacts"
for p in "${ARTIFACTS[@]}"; do
  if [ -e "$REPO_DIR/$p" ]; then
    rm -rf "${REPO_DIR:?}/$p" && ok "removed $p" || warn "could not remove $p"
  fi
done
# stray build cache
[ -f "$REPO_DIR/web/tsconfig.tsbuildinfo" ] && rm -f "$REPO_DIR/web/tsconfig.tsbuildinfo"

# ── [5/5] purge (optional): uninstall apps + config ──────────────────────────
section "Purge apps & config (optional)"
if [ "$PURGE" -eq 1 ]; then
  if [ -f "$REPO_DIR/config.json" ]; then
    cp "$REPO_DIR/config.json" "$REPO_DIR/config.json.bak" && rm -f "$REPO_DIR/config.json"
    ok "removed config.json (backup at config.json.bak)"
  fi
  if [ "$OS" = "Darwin" ] && have brew; then
    run "uninstalling MongoDB Community"; brew uninstall mongodb-community >/dev/null 2>&1 && ok "uninstalled mongodb-community" || warn "mongodb-community not installed via brew"
    run "uninstalling Ollama"; brew uninstall ollama >/dev/null 2>&1 && ok "uninstalled ollama" || warn "ollama not installed via brew"
  elif have apt-get; then
    sudo apt-get remove -y mongodb mongodb-org >/dev/null 2>&1 || true
    warn "remove Ollama manually if installed via script: sudo rm -f /usr/local/bin/ollama && sudo rm -rf ~/.ollama"
  else
    warn "purge: uninstall MongoDB/Ollama with your package manager manually"
  fi
else
  printf "   ${DIM}skipped (run with --purge to also uninstall the MongoDB & Ollama apps and delete config.json)${RST}\n"
fi

# ── done ─────────────────────────────────────────────────────────────────────
printf "\n${BOLD}${GRN}✓ Teardown complete.${RST}\n"
cat <<EOF

${DIM}Kept on purpose (shared tools / your source). To remove them yourself:${RST}
  • Repo source        : delete this folder ($REPO_DIR)
  • Go                 : brew uninstall go            ${DIM}(or your package manager)${RST}
  • Node.js            : brew uninstall node          ${DIM}(or your package manager)${RST}
  • Ollama models dir  : rm -rf ~/.ollama             ${DIM}(removes ALL models, not just this one)${RST}
  • Re-create later    : bash setup.sh
EOF
