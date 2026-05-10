#!/usr/bin/env bash
set -uo pipefail

# Send a local test message through the WhatsApp webhook endpoint and print
# the latest assistant reply from DB conversation history.
#
# Usage:
#   bash scripts/local_message_chat.sh "what all did i eat today"
#   PHONE=918309619180 BASE_URL=http://localhost:8080 bash scripts/local_message_chat.sh "delete curd"
#   bash scripts/local_message_chat.sh -i
#   bash scripts/local_message_chat.sh --replay artifacts/chat_sessions/chat_918309619180_20260510_142115.log
#   REPLAY_DELAY=0.2 bash scripts/local_message_chat.sh --replay /path/to/transcript.log

INTERACTIVE=0
REPLAY_MODE=0
REPLAY_FILE=""
REPLAY_DELAY="${REPLAY_DELAY:-0}"
RESET_USAGE_ON_REPLAY="${RESET_USAGE_ON_REPLAY:-0}"
RESET_STATE_ON_REPLAY="${RESET_STATE_ON_REPLAY:-0}"
MSG_TEXT="${1:-}"
if [ "${1:-}" = "-i" ] || [ "${1:-}" = "--interactive" ]; then
  INTERACTIVE=1
  MSG_TEXT=""
elif [ "${1:-}" = "--replay" ]; then
  REPLAY_MODE=1
  REPLAY_FILE="${2:-}"
  MSG_TEXT=""
fi

if [ $INTERACTIVE -eq 0 ] && [ $REPLAY_MODE -eq 0 ] && [ -z "$MSG_TEXT" ]; then
  echo "Usage: $0 \"message text\""
  echo "   or: $0 -i"
  echo "   or: $0 --replay /path/to/transcript.log"
  exit 1
fi
if [ $REPLAY_MODE -eq 1 ] && [ -z "$REPLAY_FILE" ]; then
  echo "Replay mode requires a transcript file path."
  echo "Usage: $0 --replay /path/to/transcript.log"
  exit 1
fi
PHONE="${PHONE:-918309619180}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APP_ENV="${APP_ENV:-development}"
if [ "$APP_ENV" = "production" ]; then
  ENV_PATH="$ROOT_DIR/.env"
else
  ENV_PATH="$ROOT_DIR/.env.development"
fi
LOG_DIR="${LOG_DIR:-$ROOT_DIR/artifacts/chat_sessions}"
ANALYZE_ON_EXIT="${ANALYZE_ON_EXIT:-1}"
mkdir -p "$LOG_DIR"
SESSION_TS="$(date +%Y%m%d_%H%M%S)"
TRANSCRIPT_FILE="${TRANSCRIPT_FILE:-$LOG_DIR/chat_${PHONE}_${SESSION_TS}.log}"

if [ ! -f "$ENV_PATH" ]; then
  echo "Missing $ENV_PATH"
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required but not installed."
  exit 1
fi

log_line() {
  local who="$1"
  local text="$2"
  printf "[%s] %s: %s\n" "$(date '+%Y-%m-%d %H:%M:%S')" "$who" "$text" >>"$TRANSCRIPT_FILE"
}

extract_db_url_from_env() {
  local env_path="$1"
  if [ ! -f "$env_path" ]; then
    return 0
  fi
  python3 - "$env_path" <<'PY'
import sys
path=sys.argv[1]
db=""
for line in open(path, encoding="utf-8"):
    s=line.strip()
    if not s or s.startswith("#"):
        continue
    if s.startswith("DATABASE_URL="):
        db=s.split("=",1)[1].strip().strip('"').strip("'")
        break
print(db)
PY
}

pick_db_url_for_phone() {
  local phone="$1"
  local primary="$2"
  local alt="$3"
  python3 - "$phone" "$primary" "$alt" <<'PY'
import datetime,subprocess,sys
phone,primary,alt=sys.argv[1],sys.argv[2],sys.argv[3]
urls=[u for u in [primary,alt] if u]
if not urls:
    print("")
    raise SystemExit(0)
def latest_ts(url):
    q=f"""
WITH u AS (
  SELECT user_id FROM user_identities
  WHERE provider='whatsapp' AND external_id='{phone}'
  LIMIT 1
)
SELECT COALESCE(to_char(MAX(updated_at), 'YYYY-MM-DD\"T\"HH24:MI:SS.US'), '')
FROM conversations
WHERE user_id IN (SELECT user_id FROM u);
"""
    try:
        out=subprocess.check_output(['psql',url,'-At','-c',q], text=True).strip()
    except Exception:
        return None
    if not out:
        return datetime.datetime.min
    try:
        return datetime.datetime.strptime(out, "%Y-%m-%dT%H:%M:%S.%f")
    except Exception:
        return datetime.datetime.min
best=urls[0]
best_ts=latest_ts(best)
for u in urls[1:]:
    ts=latest_ts(u)
    if ts is not None and best_ts is not None and ts>best_ts:
        best=u; best_ts=ts
print(best)
PY
}

send_message() {
local message_text="$1"
local replay_relaxed="${2:-0}"
local msg_id
msg_id="local-$(python3 - <<'PY'
import uuid
print(uuid.uuid4().hex)
PY
)"

local payload
payload="$(python3 - "$PHONE" "$msg_id" "$message_text" <<'PY'
import json,sys,time
phone,msg_id,text=sys.argv[1],sys.argv[2],sys.argv[3]
payload = {
  "entry": [{
    "changes": [{
      "value": {
        "messages": [{
          "from": phone,
          "id": msg_id,
          "type": "text",
          "timestamp": str(int(time.time())),
          "text": {"body": text}
        }]
      }
    }]
  }]
}
print(json.dumps(payload))
PY
)"

echo "You: $message_text"
log_line "You" "$message_text"
local db_url
primary_db_url="$(extract_db_url_from_env "$ENV_PATH")"
alt_env="$ROOT_DIR/.env"
if [ "$ENV_PATH" = "$ROOT_DIR/.env" ]; then
  alt_env="$ROOT_DIR/.env.development"
fi
alt_db_url="$(extract_db_url_from_env "$alt_env")"
db_url="$(pick_db_url_for_phone "$PHONE" "$primary_db_url" "$alt_db_url")"
if [ -z "${db_url:-}" ]; then
  echo "Could not read DATABASE_URL from env files."
  return 1
fi

local baseline
baseline="$(psql "$db_url" -At -F $'\t' -c "
WITH u AS (
  SELECT user_id FROM user_identities
  WHERE provider='whatsapp' AND external_id='${PHONE}'
  LIMIT 1
)
SELECT COALESCE(to_char(c.updated_at, 'YYYY-MM-DD\"T\"HH24:MI:SS.US'), '') || E'\t' || COALESCE(c.messages, '')
FROM conversations c, u
WHERE c.user_id=u.user_id
ORDER BY c.updated_at DESC
LIMIT 1;
")" 2>/dev/null || true
local baseline_ts baseline_msg baseline_last_assistant
baseline_ts="$(printf '%s' "$baseline" | cut -f1)"
baseline_msg="$(printf '%s' "$baseline" | cut -f2-)"
baseline_last_assistant="$(python3 - <<'PY' "$baseline_msg"
import json,sys
raw=sys.argv[1]
if not raw:
    print("")
    raise SystemExit(0)
try:
    msgs=json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
assistant=""
for m in reversed(msgs):
    if m.get("role")=="assistant":
        c=m.get("content","")
        if isinstance(c,list):
            c=" ".join([(p.get("text") or "") for p in c if isinstance(p,dict)])
        assistant=str(c).strip()
        break
print(assistant)
PY
)"

if ! curl -sS -X POST "$BASE_URL/whatsapp/webhook" \
  -H "Content-Type: application/json" \
  -d "$payload" >/dev/null; then
  echo "Failed to send webhook to $BASE_URL. Is backend running?"
  return 1
fi

# Poll async processing result.
for _ in {1..30}; do
  OUT="$(psql "$db_url" -At -F $'\t' -c "
WITH u AS (
  SELECT user_id FROM user_identities
  WHERE provider='whatsapp' AND external_id='${PHONE}'
  LIMIT 1
)
SELECT COALESCE(to_char(c.updated_at, 'YYYY-MM-DD\"T\"HH24:MI:SS.US'), '') || E'\t' || COALESCE(c.messages, '')
FROM conversations c, u
WHERE c.user_id=u.user_id
ORDER BY c.updated_at DESC
LIMIT 1;
")" 2>/dev/null || true

  if [ -n "$OUT" ]; then
    CURRENT_TS="$(printf '%s' "$OUT" | cut -f1)"
    CURRENT_MSGS="$(printf '%s' "$OUT" | cut -f2-)"
    LAST_ASSISTANT="$(python3 - <<'PY' "$CURRENT_MSGS" "$message_text" "$replay_relaxed"
import json,sys
raw=sys.argv[1]
target=(sys.argv[2] or "").strip().lower()
relaxed=(sys.argv[3] or "0")=="1"
try:
    msgs=json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
def to_text(v):
    if isinstance(v,list):
        return " ".join([(p.get("text") or "") for p in v if isinstance(p,dict)]).strip()
    return str(v or "").strip()

if relaxed:
    for m in reversed(msgs):
        if m.get("role")=="assistant":
            print(to_text(m.get("content","")))
            raise SystemExit(0)
    print("")
    raise SystemExit(0)

# Find the most recent matching user message, then first assistant after it.
match_idx=-1
for i in range(len(msgs)-1,-1,-1):
    m=msgs[i]
    if m.get("role")!="user":
        continue
    if to_text(m.get("content","")).strip().lower()==target:
        match_idx=i
        break

if match_idx>=0:
    for j in range(match_idx+1,len(msgs)):
        m=msgs[j]
        if m.get("role")=="assistant":
            print(to_text(m.get("content","")))
            raise SystemExit(0)

# Fallback to latest assistant.
for m in reversed(msgs):
    if m.get("role")=="assistant":
        print(to_text(m.get("content","")))
        raise SystemExit(0)
print("")
PY
)"
    if [ "$replay_relaxed" = "1" ]; then
      if [ -n "$LAST_ASSISTANT" ] && [ "$CURRENT_MSGS" != "$baseline_msg" ]; then
        echo "Bot: $LAST_ASSISTANT"
        log_line "Bot" "$LAST_ASSISTANT"
        return 0
      fi
    elif [ -n "$LAST_ASSISTANT" ] && { [ "$CURRENT_TS" != "$baseline_ts" ] || [ "$LAST_ASSISTANT" != "$baseline_last_assistant" ]; }; then
      echo "Bot: $LAST_ASSISTANT"
      log_line "Bot" "$LAST_ASSISTANT"
      return 0
    fi
  fi
  sleep 1
done

echo "No assistant reply found yet. Check backend logs."
if [ -n "${db_url:-}" ]; then
  local limit_diag
  limit_diag="$(python3 - "$db_url" "$PHONE" <<'PY'
import subprocess,sys
db,phone=sys.argv[1],sys.argv[2]
def q(sql):
    try:
        return subprocess.check_output(['psql',db,'-At','-c',sql], text=True).strip()
    except Exception:
        return ""
uid=q(f"select user_id from user_identities where provider='whatsapp' and external_id='{phone}' limit 1;")
if not uid:
    print("diag: no user identity found for phone")
    raise SystemExit(0)
count=q(f"select count(*) from llm_usage_logs where user_id={uid} and created_at::date=now()::date;") or "0"
try:
    limit=q("select current_setting('app.whatsapp_daily_limit', true);")
except Exception:
    limit=""
print(f"diag: user_id={uid}, llm_usage_today={count}, daily_limit_env={limit or 'unknown'}")
PY
)"
  if [ -n "$limit_diag" ]; then
    echo "$limit_diag"
  fi
fi
return 1
}

print_analysis() {
  if [ "$ANALYZE_ON_EXIT" = "1" ] && [ -f "$TRANSCRIPT_FILE" ]; then
    echo ""
    echo "Conversation analysis:"
    python3 - "$TRANSCRIPT_FILE" <<'PY'
import sys,re
p=sys.argv[1]
lines=[l.rstrip("\n") for l in open(p,encoding="utf-8",errors="ignore")]
you=sum(1 for l in lines if "] You:" in l)
bot=sum(1 for l in lines if "] Bot:" in l)
json_leak=sum(1 for l in lines if "] Bot:" in l and re.search(r'\{.*\}', l))
action_leak=sum(1 for l in lines if "] Bot:" in l and "Action:" in l)
fallback=sum(1 for l in lines if "] Bot:" in l and "i heard you, but i'm not sure how to response" in l.lower())
print(f"- file: {p}")
print(f"- user messages: {you}")
print(f"- bot replies: {bot}")
print(f"- possible JSON leaks: {json_leak}")
print(f"- Action leaks: {action_leak}")
print(f"- fallback phrase hits: {fallback}")
PY
  fi
}

if [ $INTERACTIVE -eq 1 ]; then
  if [ ! -t 0 ] && [ ! -e /dev/tty ]; then
    echo "Interactive mode requires a terminal (TTY)."
    exit 1
  fi
  echo "Interactive mode. Type /exit to quit."
  echo "Transcript: $TRANSCRIPT_FILE"
  while true; do
    printf "You> "
    if [ -t 0 ]; then
      IFS= read -r line || break
    else
      IFS= read -r line </dev/tty || break
    fi
    if [ -z "$line" ]; then
      continue
    fi
    if [ "$line" = "/exit" ] || [ "$line" = "/quit" ]; then
      break
    fi
    send_message "$line" 1 || true
  done
  print_analysis
  exit 0
fi

if [ $REPLAY_MODE -eq 1 ]; then
  if [ ! -f "$REPLAY_FILE" ]; then
    echo "Transcript file not found: $REPLAY_FILE"
    exit 1
  fi
  echo "Replay mode. Source: $REPLAY_FILE"
  echo "Transcript: $TRANSCRIPT_FILE"
  if [ "$RESET_USAGE_ON_REPLAY" = "1" ]; then
    DB_URL="$(python3 - "$ENV_PATH" <<'PY'
import sys
path=sys.argv[1]
db=""
for line in open(path, encoding="utf-8"):
    s=line.strip()
    if s.startswith("DATABASE_URL="):
        db=s.split("=",1)[1].strip().strip('"').strip("'")
        break
print(db)
PY
)"
    if [ -n "$DB_URL" ]; then
      psql "$DB_URL" -v ON_ERROR_STOP=1 -c "
WITH u AS (
  SELECT user_id FROM user_identities WHERE provider='whatsapp' AND external_id='${PHONE}' LIMIT 1
)
DELETE FROM llm_usage_logs
WHERE user_id IN (SELECT user_id FROM u)
  AND created_at::date = now()::date;
" >/dev/null 2>&1 || true
      echo "Reset today's LLM usage logs for replay phone: $PHONE"
    fi
  fi
  if [ "$RESET_STATE_ON_REPLAY" = "1" ]; then
    DB_URL="$(extract_db_url_from_env "$ENV_PATH")"
    if [ -n "$DB_URL" ]; then
      psql "$DB_URL" -v ON_ERROR_STOP=1 -c "
WITH u AS (
  SELECT user_id FROM user_identities WHERE provider='whatsapp' AND external_id='${PHONE}' LIMIT 1
)
DELETE FROM conversations WHERE user_id IN (SELECT user_id FROM u);
WITH u AS (
  SELECT user_id FROM user_identities WHERE provider='whatsapp' AND external_id='${PHONE}' LIMIT 1
)
DELETE FROM conversation_states WHERE user_id IN (SELECT user_id FROM u);
" >/dev/null 2>&1 || true
      echo "Reset conversation state for replay phone: $PHONE"
    fi
  fi
  python3 - "$REPLAY_FILE" <<'PY' | while IFS= read -r line; do
import re,sys
path=sys.argv[1]
for raw in open(path, encoding="utf-8", errors="ignore"):
    m=re.search(r'\]\s+You:\s+(.*)\s*$', raw.rstrip('\n'))
    if not m:
        continue
    msg=m.group(1).strip()
    if msg:
        print(msg)
PY
    send_message "$line" || true
    if [ "$REPLAY_DELAY" != "0" ]; then
      sleep "$REPLAY_DELAY"
    fi
  done
  print_analysis
  exit 0
fi

send_message "$MSG_TEXT"
