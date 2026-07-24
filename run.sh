#!/usr/bin/env bash
#
# run.sh — start both backend and frontend servers for rejected.ai.
#
#   Usage:   bash run.sh
#

set -uo pipefail

# ── pretty output (matches setup.sh / teardown.sh) ───────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GRN=$'\033[32m'
  YLW=$'\033[33m'; CYN=$'\033[36m'; RST=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GRN=""; YLW=""; CYN=""; RST=""
fi

# Locate repo root
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

OS="$(uname -s)"
printf "${BOLD}rejected.ai runner${RST}  ${DIM}(%s)${RST}\n" "$OS"
printf "${DIM}Repo: %s${RST}\n" "$REPO_DIR"

have() { command -v "$1" >/dev/null 2>&1; }

# Check config
if [ ! -f config.json ]; then
  printf "${RED}✗ config.json not found.${RST} Please run ${BOLD}./setup.sh${RST} first.\n"
  exit 1
fi

# Parse config values
DB_URI="$(grep -oE '"MONGO_URI"[[:space:]]*:[[:space:]]*"[^"]+"' config.json | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
DB_URI="${DB_URI:-mongodb://localhost:27017}"

LLM_BACKEND="$(grep -oE '"LLM_BACKEND"[[:space:]]*:[[:space:]]*"[^"]+"' config.json | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
LLM_BACKEND="${LLM_BACKEND:-ollama}"

OLLAMA_HOST="$(grep -oE '"OLLAMA_HOST"[[:space:]]*:[[:space:]]*"[^"]+"' config.json | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"

# Check/Start services
# Extract host and port from DB_URI (usually localhost:27017)
MONGO_PORT="27017"
if [[ "$DB_URI" =~ :([0-9]+) ]]; then
  MONGO_PORT="${BASH_REMATCH[1]}"
fi

# Helper to check if a port is responding
check_port() {
  local host="$1" port="$2"
  (exec 3<>/dev/tcp/"$host"/"$port") 2>/dev/null && { exec 3>&- 3<&- 2>/dev/null; return 0; } || return 1
}

# Wait until a TCP port answers
wait_for_port() {
  local host="$1" port="$2" label="$3" tries="${4:-15}"
  local spin='|/-\' i=0
  printf "   ${BOLD}▶${RST} waiting for %s (%s:%s) " "$label" "$host" "$port"
  while [ "$tries" -gt 0 ]; do
    if check_port "$host" "$port"; then
      printf "\r   ${GRN}✓ %s is up (%s:%s)            ${RST}\n" "$label" "$host" "$port"
      return 0
    fi
    i=$(((i + 1) % 4)); printf "\b%s" "${spin:$i:1}"; sleep 1; tries=$((tries - 1))
  done
  printf "\r   ${YLW}! %s not reachable on %s:%s yet${RST}\n" "$label" "$host" "$port"
  return 1
}

# 1. MongoDB check/start
printf "\n${BOLD}${CYN}━━ Checking MongoDB ━━${RST}\n"
if check_port localhost "$MONGO_PORT"; then
  printf "   ${GRN}✓ MongoDB is running on port %s${RST}\n" "$MONGO_PORT"
else
  printf "   ${YLW}! MongoDB is not running on port %s. Trying to start...${RST}\n" "$MONGO_PORT"
  if [ "$OS" = "Darwin" ] && have brew; then
    brew services start mongodb-community || true
  elif have docker && docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q '^rejected-mongo$'; then
    docker start rejected-mongo || true
  elif [ "$OS" != "Darwin" ]; then
    sudo systemctl start mongod 2>/dev/null || sudo systemctl start mongodb 2>/dev/null || true
  fi
  wait_for_port localhost "$MONGO_PORT" "MongoDB" 15 || {
    printf "${RED}✗ MongoDB failed to start. Please start MongoDB manually and run again.${RST}\n"
    exit 1
  }
fi

# 2. Ollama check/start (if ollama backend is active)
if [ "$LLM_BACKEND" = "ollama" ]; then
  printf "\n${BOLD}${CYN}━━ Checking Ollama ━━${RST}\n"
  # Extract host and port from OLLAMA_HOST (usually http://localhost:11434)
  OLLAMA_PORT="11434"
  if [[ "$OLLAMA_HOST" =~ :([0-9]+) ]]; then
    OLLAMA_PORT="${BASH_REMATCH[1]}"
  fi
  
  if check_port localhost "$OLLAMA_PORT"; then
    printf "   ${GRN}✓ Ollama is running on port %s${RST}\n" "$OLLAMA_PORT"
  else
    printf "   ${YLW}! Ollama is not running on port %s. Trying to start...${RST}\n" "$OLLAMA_PORT"
    if [ "$OS" = "Darwin" ] && have brew; then
      brew services start ollama >/dev/null 2>&1 || (nohup ollama serve >/tmp/ollama.log 2>&1 &)
    else
      (nohup ollama serve >/tmp/ollama.log 2>&1 &)
    fi
    wait_for_port localhost "$OLLAMA_PORT" "Ollama" 15 || {
      printf "${RED}✗ Ollama failed to start. Please run 'ollama serve' manually and run again.${RST}\n"
      exit 1
    }
  fi
fi

# 3. Build Backend
printf "\n${BOLD}${CYN}━━ Building Backend ━━${RST}\n"
if ! have go; then
  printf "${RED}✗ Go toolchain not found. Install Go and run setup.sh.${RST}\n"
  exit 1
fi

printf "   ${BOLD}▶${RST} go build -o bin/server ./cmd/server\n"
if go build -o bin/server ./cmd/server; then
  printf "   ${GRN}✓ Backend built successfully.${RST}\n"
else
  printf "${RED}✗ Backend build failed.${RST}\n"
  exit 1
fi

# 4. Start servers
printf "\n${BOLD}${CYN}━━ Starting Servers ━━${RST}\n"

BACKEND_PID=""
FRONTEND_PID=""

cleanup() {
  echo ""
  printf "\n${BOLD}${YLW}Stopping servers...${RST}\n"
  if [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null
  fi
  if [ -n "$FRONTEND_PID" ]; then
    kill "$FRONTEND_PID" 2>/dev/null
  fi
  wait
  printf "${BOLD}${GRN}✓ Stopped.${RST}\n"
}

# Trap exit/interrupt signals to ensure cleanup
trap cleanup EXIT SIGINT SIGTERM

# Parse backend port
SERVER_ADDR="$(grep -oE '"SERVER_ADDR"[[:space:]]*:[[:space:]]*"[^"]+"' config.json | head -1 | sed -E 's/.*"([^"]+)"$/\1/')"
SERVER_ADDR="${SERVER_ADDR:-:8090}"
BACKEND_PORT="8090"
if [[ "$SERVER_ADDR" =~ :([0-9]+) ]]; then
  BACKEND_PORT="${BASH_REMATCH[1]}"
fi

mkdir -p logs
printf "   ${GRN}Backend starting...${RST}\n"
./bin/server > logs/backend.log 2>&1 &
BACKEND_PID=$!

# Wait for backend to be listening (or 3 seconds maximum)
backend_up=0
for i in {1..30}; do
  if check_port localhost "$BACKEND_PORT"; then
    backend_up=1
    break
  fi
  sleep 0.1
done

if [ "$backend_up" -eq 1 ]; then
  printf "   ${GRN}✓ Backend running at http://localhost:%s (logs -> logs/backend.log)${RST}\n" "$BACKEND_PORT"
else
  printf "   ${YLW}! Backend started but not responding on port %s yet (check logs/backend.log)${RST}\n" "$BACKEND_PORT"
fi

printf "   ${GRN}Frontend starting...${RST}\n"
if ! have npm; then
  printf "${RED}✗ npm not found. Install Node.js/npm and run setup.sh.${RST}\n"
  exit 1
fi

(cd web && npm run dev) &
FRONTEND_PID=$!
printf "   ${GRN}✓ Frontend dev server starting at http://localhost:3000${RST}\n\n"

# Monitor the processes
while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 1
done
