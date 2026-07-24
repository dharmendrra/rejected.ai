#!/usr/bin/env bash
#
# setup.sh — one-shot setup for rejected.ai.
#
#   Usage:   bash setup.sh
#
# Installs everything the project needs (Go, Node, MongoDB, Ollama + the model,
# Go modules, frontend npm deps) and leaves you ready to run. It does NOT start
# the servers or build — it prints those two commands at the end for you to copy.
#
# Safe to re-run: every run re-checks/re-installs/updates everything (the package
# managers and `ollama pull` are idempotent, so repeats are fast).
#
# Long downloads (Homebrew/apt packages, the ~9 GB Ollama model, npm, Go modules)
# stream their OWN native progress — percentages and download speed — because this
# script intentionally does not hide their output.

set -uo pipefail

# Skip the confirmation prompt with:  bash setup.sh -y   (or --yes)
ASSUME_YES=0
for a in "$@"; do case "$a" in -y|--yes) ASSUME_YES=1 ;; esac; done

# ── pretty output ────────────────────────────────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GRN=$'\033[32m'
  YLW=$'\033[33m'; CYN=$'\033[36m'; RST=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GRN=""; YLW=""; CYN=""; RST=""
fi

STEP=0
TOTAL=9
START_TS=$(date +%s)

section() { STEP=$((STEP + 1)); printf "\n${BOLD}${CYN}━━ [%d/%d] %s ━━${RST}\n" "$STEP" "$TOTAL" "$1"; }
info()    { printf "   ${DIM}%s${RST}\n" "$1"; }
run()     { printf "   ${BOLD}▶${RST} %s\n" "$1"; }
ok()      { printf "   ${GRN}✓ %s${RST}\n" "$1"; }
warn()    { printf "   ${YLW}! %s${RST}\n" "$1"; }
die()     { printf "\n${RED}✗ %s${RST}\n" "$1" >&2; exit 1; }

have()    { command -v "$1" >/dev/null 2>&1; }

# Wait until a TCP port answers, showing a live spinner so there's always feedback.
wait_for_port() {
  local host="$1" port="$2" label="$3" tries="${4:-60}"
  local spin='|/-\' i=0
  printf "   ${BOLD}▶${RST} waiting for %s (%s:%s) " "$label" "$host" "$port"
  while [ "$tries" -gt 0 ]; do
    if (exec 3<>"/dev/tcp/$host/$port") 2>/dev/null; then
      exec 3>&- 3<&- 2>/dev/null
      printf "\r   ${GRN}✓ %s is up (%s:%s)            ${RST}\n" "$label" "$host" "$port"
      return 0
    fi
    i=$(((i + 1) % 4)); printf "\b%s" "${spin:$i:1}"; sleep 1; tries=$((tries - 1))
  done
  printf "\r   ${YLW}! %s not reachable on %s:%s yet${RST}\n" "$label" "$host" "$port"
  return 1
}

# ── locate repo root (this script lives at the repo root) ─────────────────────
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_DIR"

OS="$(uname -s)"
printf "${BOLD}rejected.ai setup${RST}  ${DIM}(%s)${RST}\n" "$OS"
printf "${DIM}Repo: %s${RST}\n" "$REPO_DIR"
printf "${DIM}Heavy downloads show their own %% and speed below. You may be asked for your password (sudo) for system installs.${RST}\n"

# ── package-manager abstraction ──────────────────────────────────────────────
PKG=""
if [ "$OS" = "Darwin" ]; then
  PKG="brew"
elif have apt-get; then
  PKG="apt"
elif have dnf; then
  PKG="dnf"
fi

pkg_install() { # pkg_install <human label> <brew formula> <apt pkg> <dnf pkg>
  local label="$1" brewp="$2" aptp="$3" dnfp="$4"
  run "installing/updating $label"
  case "$PKG" in
    brew) brew install "$brewp" || brew upgrade "$brewp" || true ;;
    apt)  sudo apt-get update -y && sudo apt-get install -y $aptp ;;
    dnf)  sudo dnf install -y $dnfp ;;
    *)    warn "no supported package manager found — install $label manually"; return 1 ;;
  esac
}

# ── estimate what needs downloading, then ask permission ─────────────────────
# Which model the backend uses (config.json may not exist yet → default).
MODEL="$(grep -oE '"OLLAMA_MODEL"[[:space:]]*:[[:space:]]*"[^"]+"' config.json 2>/dev/null | sed -E 's/.*"([^"]+)"$/\1/')"
MODEL="${MODEL:-gemma4:e4b}"

gb() { awk -v mb="$1" 'BEGIN{ printf "%.1f", mb/1024 }'; }
mongo_running() { (exec 3<>/dev/tcp/localhost/27017) 2>/dev/null && { exec 3>&- 3<&- 2>/dev/null; return 0; } || return 1; }
model_present() { have ollama && ollama list 2>/dev/null | awk 'NR>1{print $1}' | grep -q "^${MODEL%%:*}"; }

PLAN_NAMES=(); PLAN_MB=(); TOTAL_MB=0
plan_add() { PLAN_NAMES+=("$1"); PLAN_MB+=("$2"); TOTAL_MB=$((TOTAL_MB + $2)); }

[ "$OS" = "Darwin" ] && ! have brew && plan_add "Homebrew (package manager)" 300
have go            || plan_add "Go toolchain" 200
{ have node && have npm; } || plan_add "Node.js + npm" 120
mongo_running      || plan_add "MongoDB Community + start" 500
have ollama        || plan_add "Ollama runtime" 500
model_present      || plan_add "LLM model ${MODEL} (the big one)" 9600
plan_add "Frontend npm packages (web/)" 400
plan_add "Go modules" 60

printf "\n${BOLD}This setup will download & install:${RST}\n"
if [ "${#PLAN_NAMES[@]}" -eq 0 ]; then
  printf "   ${GRN}nothing — everything already present${RST}\n"
else
  for i in "${!PLAN_NAMES[@]}"; do
    printf "   • %-40s ~%s GB\n" "${PLAN_NAMES[$i]}" "$(gb "${PLAN_MB[$i]}")"
  done
fi
TOTAL_GB="$(gb "$TOTAL_MB")"
printf "${BOLD}   ──────────────────────────────────────────────${RST}\n"
printf "   ${BOLD}Estimated total download: ~%s GB${RST}  ${DIM}(rough; already-installed items are skipped)${RST}\n" "$TOTAL_GB"
model_present || printf "   ${DIM}Most of that is the one-time ~9.6 GB model.${RST}\n"
printf "   ${DIM}May also prompt for your password (sudo) for system installs.${RST}\n"

if [ "$ASSUME_YES" -ne 1 ]; then
  printf "\n${BOLD}Proceed? [y/N] ${RST}"
  REPLY=""
  got=1
  # Prefer /dev/tty (works even when stdin is piped); fall back to stdin if it's
  # a terminal. We treat a FAILED read (no usable terminal) differently from a
  # successful read of "no" — so a non-interactive run gets a clear hint, not a
  # silent abort. 2>/dev/null hides the "Device not configured" noise.
  if read -r REPLY < /dev/tty 2>/dev/null; then
    got=0
  elif [ -t 0 ] && read -r REPLY; then
    got=0
  fi
  if [ "$got" -ne 0 ]; then
    printf "\n${YLW}! No interactive terminal detected, so I can't ask for confirmation.${RST}\n"
    printf "${DIM}  Re-run in a real terminal, or skip the prompt with:${RST} ${BOLD}./setup.sh -y${RST}\n"
    exit 1
  fi
  case "$REPLY" in
    y | Y | yes | YES) : ;;
    *) printf "${YLW}Aborted — nothing was installed.${RST}\n"; exit 0 ;;
  esac
fi

# ── [1/9] package manager ────────────────────────────────────────────────────
section "Package manager"
if [ "$OS" = "Darwin" ]; then
  if have brew; then ok "Homebrew present ($(brew --version | head -1))"
  else
    run "installing Homebrew (shows its own progress)"
    NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" \
      || die "Homebrew install failed"
    # make brew available on Apple Silicon in this shell
    [ -x /opt/homebrew/bin/brew ] && eval "$(/opt/homebrew/bin/brew shellenv)"
    ok "Homebrew installed"
  fi
elif [ -n "$PKG" ]; then
  ok "using system package manager: $PKG"
else
  warn "unrecognized OS/package manager — best-effort; some installs may be skipped"
fi

# ── [2/9] Go ─────────────────────────────────────────────────────────────────
section "Go toolchain"
if have go; then ok "Go present ($(go version | awk '{print $3}'))"
else
  pkg_install "Go" go golang-go golang || true
  have go && ok "Go installed ($(go version | awk '{print $3}'))" || warn "Go not on PATH — install from https://go.dev/dl/"
fi

# ── [3/9] Node + npm ─────────────────────────────────────────────────────────
section "Node.js + npm"
if have node && have npm; then ok "Node present ($(node -v)), npm ($(npm -v))"
else
  pkg_install "Node.js" node nodejs npm || true
  have node && ok "Node installed ($(node -v))" || warn "Node not on PATH — install Node 20+ from https://nodejs.org"
fi

# ── [4/9] MongoDB ────────────────────────────────────────────────────────────
section "MongoDB (port 27017)"
if (exec 3<>/dev/tcp/localhost/27017) 2>/dev/null; then
  exec 3>&- 3<&- 2>/dev/null; ok "MongoDB already running"
else
  if [ "$OS" = "Darwin" ]; then
    run "installing MongoDB Community via Homebrew"
    brew tap mongodb/brew >/dev/null 2>&1 || true
    brew install mongodb-community || brew upgrade mongodb-community || true
    run "starting MongoDB service"
    brew services start mongodb-community || true
  elif have docker; then
    run "starting MongoDB via Docker (container: rejected-mongo)"
    docker start rejected-mongo >/dev/null 2>&1 || \
      docker run -d --name rejected-mongo -p 27017:27017 mongo:7 >/dev/null 2>&1 || true
  elif [ "$PKG" = "apt" ]; then
    pkg_install "MongoDB" "" mongodb "" || true
    sudo systemctl enable --now mongodb 2>/dev/null || sudo systemctl enable --now mongod 2>/dev/null || true
  else
    warn "could not auto-install MongoDB — install it or run: docker run -d -p 27017:27017 mongo:7"
  fi
  wait_for_port localhost 27017 "MongoDB" 60 || warn "start MongoDB manually, then re-run."
fi

# ── [5/9] Ollama (port 11434) ────────────────────────────────────────────────
section "Ollama (local LLM, port 11434)"
if ! have ollama; then
  if [ "$OS" = "Darwin" ]; then
    brew install ollama || true
  else
    run "installing Ollama (shows its own progress)"
    curl -fsSL https://ollama.com/install.sh | sh || true
  fi
fi
if (exec 3<>/dev/tcp/localhost/11434) 2>/dev/null; then
  exec 3>&- 3<&- 2>/dev/null; ok "Ollama already serving"
else
  run "starting the Ollama server in the background"
  if [ "$OS" = "Darwin" ] && have brew; then
    brew services start ollama >/dev/null 2>&1 || (nohup ollama serve >/tmp/ollama.log 2>&1 &)
  else
    (nohup ollama serve >/tmp/ollama.log 2>&1 &)
  fi
  wait_for_port localhost 11434 "Ollama" 60 || warn "start it with: ollama serve"
fi

# ── [6/9] Generation model ───────────────────────────────────────────────────
section "LLM model ($MODEL — large, ~9 GB)"
if have ollama; then
  run "pulling $MODEL (download progress + speed shown by ollama)"
  ollama pull "$MODEL" || warn "model pull failed — re-run later: ollama pull $MODEL"
  ok "model ready: $MODEL"
else
  warn "ollama unavailable — later run: ollama pull $MODEL"
fi

# ── [7/9] config.json ────────────────────────────────────────────────────────
section "Configuration"
if [ -f config.json ]; then ok "config.json already exists (left untouched)"
else
  cp config.example.json config.json && ok "created config.json from config.example.json"
fi

# ── [8/9] Go modules ─────────────────────────────────────────────────────────
section "Go modules"
if have go; then
  run "downloading Go modules"
  go mod download && ok "Go modules ready" || warn "go mod download failed"
else
  warn "skipped — Go not installed"
fi

# ── [9/9] Frontend dependencies ──────────────────────────────────────────────
section "Frontend dependencies (web/)"
if have npm; then
  run "npm install (progress + speed shown by npm)"
  ( cd web && npm install ) && ok "frontend deps installed" || warn "npm install failed"
else
  warn "skipped — npm not installed"
fi

# ── done ─────────────────────────────────────────────────────────────────────
ELAPSED=$(( $(date +%s) - START_TS ))
printf "\n${BOLD}${GRN}✓ Setup complete in %dm %ds.${RST}\n" "$((ELAPSED / 60))" "$((ELAPSED % 60))"
cat <<EOF

${BOLD}Now start the app yourself — two terminals:${RST}

  ${BOLD}Terminal 1 — backend (build + run):${RST}
    ${CYN}cd "$REPO_DIR" && go build -o bin/server ./cmd/server && ./bin/server${RST}

  ${BOLD}Terminal 2 — frontend (Next.js dev server):${RST}
    ${CYN}cd "$REPO_DIR/web" && npm run dev${RST}

Then open ${BOLD}http://localhost:3000${RST}
(backend health: ${DIM}curl http://localhost:8090/healthz${RST})
EOF
