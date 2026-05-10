#!/usr/bin/env python3
import argparse
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request
import uuid
from dataclasses import dataclass
from typing import List, Optional, Tuple


@dataclass
class Turn:
    user: str
    bot: Optional[str] = None


def parse_transcript(path: str) -> List[Turn]:
    turns: List[Turn] = []
    user_re = re.compile(r"\]\s+You:\s+(.*)\s*$")
    bot_re = re.compile(r"\]\s+Bot:\s+(.*)\s*$")
    for raw in open(path, encoding="utf-8", errors="ignore"):
        line = raw.rstrip("\n")
        um = user_re.search(line)
        if um:
            turns.append(Turn(user=um.group(1).strip()))
            continue
        bm = bot_re.search(line)
        if bm and turns and turns[-1].bot is None:
            turns[-1].bot = bm.group(1).strip()
    return [t for t in turns if t.user]


def extract_db_url(env_path: str) -> str:
    if not os.path.exists(env_path):
        return ""
    for raw in open(env_path, encoding="utf-8", errors="ignore"):
        s = raw.strip()
        if not s or s.startswith("#"):
            continue
        if s.startswith("DATABASE_URL="):
            return s.split("=", 1)[1].strip().strip('"').strip("'")
    return ""


def run_psql(db_url: str, sql: str) -> str:
    try:
        out = subprocess.check_output(
            ["psql", db_url, "-At", "-F", "\t", "-c", sql],
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return out.strip()
    except Exception:
        return ""


def get_conversation_snapshot(db_url: str, phone: str) -> Tuple[str, str]:
    sql = f"""
WITH u AS (
  SELECT user_id FROM user_identities
  WHERE provider='whatsapp' AND external_id='{phone}'
  LIMIT 1
)
SELECT COALESCE(to_char(c.updated_at, 'YYYY-MM-DD"T"HH24:MI:SS.US'), '') || E'\\t' || COALESCE(c.messages, '')
FROM conversations c, u
WHERE c.user_id=u.user_id
ORDER BY c.updated_at DESC
LIMIT 1;
"""
    out = run_psql(db_url, sql)
    if "\t" not in out:
        return "", ""
    ts, msgs = out.split("\t", 1)
    return ts, msgs


def last_assistant_after_user(messages_json: str, user_text: str) -> str:
    if not messages_json:
        return ""
    try:
        msgs = json.loads(messages_json)
    except Exception:
        return ""

    def to_text(v):
        if isinstance(v, list):
            return " ".join(
                [str(p.get("text", "")).strip() for p in v if isinstance(p, dict)]
            ).strip()
        return str(v or "").strip()

    target = user_text.strip().lower()
    match_idx = -1
    for i in range(len(msgs) - 1, -1, -1):
        m = msgs[i]
        if m.get("role") != "user":
            continue
        if to_text(m.get("content", "")).strip().lower() == target:
            match_idx = i
            break
    if match_idx >= 0:
        for j in range(match_idx + 1, len(msgs)):
            m = msgs[j]
            if m.get("role") == "assistant":
                return to_text(m.get("content", ""))

    for m in reversed(msgs):
        if m.get("role") == "assistant":
            return to_text(m.get("content", ""))
    return ""


def send_webhook(base_url: str, phone: str, text: str, msg_id: Optional[str] = None) -> bool:
    if not msg_id:
        msg_id = "local-" + uuid.uuid4().hex
    payload = {
        "entry": [
            {
                "changes": [
                    {
                        "value": {
                            "messages": [
                                {
                                    "from": phone,
                                    "id": msg_id,
                                    "type": "text",
                                    "timestamp": str(int(time.time())),
                                    "text": {"body": text},
                                }
                            ]
                        }
                    }
                ]
            }
        ]
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url=base_url.rstrip("/") + "/whatsapp/webhook",
        data=data,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=8) as resp:
            return 200 <= resp.status < 300
    except urllib.error.URLError:
        return False


def normalize(s: str) -> str:
    return re.sub(r"\s+", " ", (s or "").strip().lower())


def replay(
    turns: List[Turn],
    base_url: str,
    phone: str,
    db_url: str,
    timeout_sec: int,
    delay_sec: float,
    check_expected: bool,
) -> int:
    failures = 0
    for idx, turn in enumerate(turns, start=1):
        print(f"You> {turn.user}")
        baseline_ts, baseline_msgs = get_conversation_snapshot(db_url, phone)
        baseline_assistant = last_assistant_after_user(baseline_msgs, turn.user)

        if not send_webhook(base_url, phone, turn.user):
            print(f"Bot: [ERROR] Failed to send webhook to {base_url}")
            failures += 1
            continue

        bot_reply = ""
        started = time.time()
        while time.time() - started < timeout_sec:
            cur_ts, cur_msgs = get_conversation_snapshot(db_url, phone)
            candidate = last_assistant_after_user(cur_msgs, turn.user)
            changed = (cur_ts != baseline_ts) or (candidate and candidate != baseline_assistant)
            if candidate and changed:
                bot_reply = candidate
                break
            time.sleep(1)

        if not bot_reply:
            print("Bot: [TIMEOUT] No assistant reply found")
            failures += 1
            continue

        print(f"Bot: {bot_reply}")
        if check_expected and turn.bot is not None:
            if normalize(turn.bot) != normalize(bot_reply):
                failures += 1
                print(f"  MISMATCH[{idx}] expected: {turn.bot}")
                print(f"  MISMATCH[{idx}] actual  : {bot_reply}")

        if delay_sec > 0:
            time.sleep(delay_sec)

    return failures


def main() -> int:
    ap = argparse.ArgumentParser(description="Replay a chat transcript against local WhatsApp webhook.")
    ap.add_argument("transcript", help="Path to transcript log file")
    ap.add_argument("--base-url", default=os.getenv("BASE_URL", "http://localhost:8080"))
    ap.add_argument("--phone", default=os.getenv("PHONE", "918309619180"))
    ap.add_argument("--env-file", default=os.getenv("ENV_FILE", ".env.development"))
    ap.add_argument("--db-url", default=os.getenv("DATABASE_URL", ""))
    ap.add_argument("--timeout-sec", type=int, default=30)
    ap.add_argument("--delay-sec", type=float, default=0.0)
    ap.add_argument("--check-expected", action="store_true", help="Compare replay reply with transcript bot line.")
    args = ap.parse_args()

    turns = parse_transcript(args.transcript)
    if not turns:
        print(f"No user turns parsed from: {args.transcript}")
        return 1

    db_url = args.db_url or extract_db_url(args.env_file)
    if not db_url:
        print("DATABASE_URL not found. Pass --db-url or set in env file.")
        return 1

    print(f"Replay turns: {len(turns)}")
    print(f"Base URL: {args.base_url}")
    print(f"Phone: {args.phone}")
    failures = replay(
        turns=turns,
        base_url=args.base_url,
        phone=args.phone,
        db_url=db_url,
        timeout_sec=args.timeout_sec,
        delay_sec=args.delay_sec,
        check_expected=args.check_expected,
    )
    print(f"Replay done. failures={failures}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())

