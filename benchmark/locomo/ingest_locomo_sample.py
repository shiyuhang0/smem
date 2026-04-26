#!/usr/bin/env python3

import argparse
import json
import time
import urllib.error
import urllib.request
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Preprocess and ingest one LoCoMo sample into smem."
    )
    parser.add_argument("--input", required=True, help="Path to one-sample LoCoMo JSON file")
    parser.add_argument(
        "--base-url",
        default="http://localhost:8080",
        help="smem server base URL",
    )
    parser.add_argument(
        "--mode",
        default="smart",
        help="smem ingest mode to write into each request",
    )
    parser.add_argument(
        "--output-dir",
        default="benchmark/locomo/out/sample_ingest",
        help="Directory for generated requests and simple ingest state",
    )
    parser.add_argument(
        "--pause-ms",
        type=int,
        default=0,
        help="Pause between requests in milliseconds",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="Only ingest the first N requests from this sample; 0 means all",
    )
    parser.add_argument(
        "--progress-every",
        type=int,
        default=50,
        help="Print progress every N requests",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=int,
        default=30,
        help="HTTP timeout per request",
    )
    parser.add_argument(
        "--start-index",
        type=int,
        default=0,
        help="1-based request index to start from; 0 means auto resume from state file",
    )
    return parser.parse_args()


def count_conversation(sample: dict) -> tuple[int, int]:
    sessions = 0
    turns = 0
    for key, value in sample["conversation"].items():
        if key.startswith("session_") and not key.endswith("_date_time"):
            sessions += 1
            turns += len(value)
    return sessions, turns


def get_session_numbers(conversation: dict) -> list[int]:
    session_numbers: list[int] = []
    for key in conversation:
        if not key.startswith("session_") or key.endswith("_date_time"):
            continue
        suffix = key.removeprefix("session_")
        if suffix.isdigit():
            session_numbers.append(int(suffix))
    return sorted(session_numbers)


def build_content(session_date_time: str, turn: dict) -> str:
    lines = [session_date_time, f"{turn['speaker']}: {turn['text']}"]
    blip_caption = turn.get("blip_caption")
    if blip_caption:
        lines.append(f"Image: {blip_caption}")
    return "\n".join(lines)


def build_requests(sample: dict, mode: str) -> list[dict]:
    requests: list[dict] = []
    conversation = sample["conversation"]
    for session_num in get_session_numbers(conversation):
        session_date_time = conversation[f"session_{session_num}_date_time"]
        for turn in conversation[f"session_{session_num}"]:
            requests.append({"content": build_content(session_date_time, turn), "mode": mode})
    return requests


def post_json(url: str, payload: dict, timeout_seconds: int) -> tuple[int, dict]:
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:
        body = resp.read().decode("utf-8")
        return resp.status, json.loads(body) if body else {}


def load_state(state_path: Path) -> dict:
    if not state_path.exists():
        return {}
    return json.loads(state_path.read_text(encoding="utf-8"))


def save_state(state_path: Path, state: dict) -> None:
    state_path.parent.mkdir(parents=True, exist_ok=True)
    state_path.write_text(json.dumps(state, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    input_path = Path(args.input)
    output_dir = Path(args.output_dir)

    samples = json.loads(input_path.read_text(encoding="utf-8"))
    if len(samples) != 1:
        raise SystemExit("input must contain exactly one sample")

    sample = samples[0]
    sample_id = sample["sample_id"]
    sessions, turns = count_conversation(sample)
    requests = build_requests(sample, args.mode)

    sample_dir = output_dir / sample_id
    sample_dir.mkdir(parents=True, exist_ok=True)
    requests_path = sample_dir / "requests.jsonl"
    manifest_path = sample_dir / "manifest.json"
    state_path = sample_dir / "ingest_state.json"

    with requests_path.open("w", encoding="utf-8") as fh:
        for item in requests:
            fh.write(json.dumps(item, ensure_ascii=False) + "\n")

    manifest = {
        "sample_id": sample_id,
        "input": str(input_path),
        "requests": str(requests_path),
        "sessions": sessions,
        "turns": turns,
        "qas": len(sample.get("qa", [])),
    }
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    print(json.dumps(manifest, ensure_ascii=False, indent=2))

    saved_state = load_state(state_path)
    resume_index = args.start_index or int(saved_state.get("next_index", 1))
    if resume_index < 1:
        resume_index = 1

    if args.limit > 0:
        requests = requests[: args.limit]

    if resume_index > len(requests):
        print(
            json.dumps(
                {
                    "sample_id": sample_id,
                    "requested": len(requests),
                    "message": "Nothing to ingest; start index is beyond input length.",
                    "start_index": resume_index,
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 0

    ingest_url = args.base_url.rstrip("/") + "/api/v1/memories"
    accepted = 0
    failed = 0
    started_at = time.time()

    print(
        json.dumps(
            {
                "status": "starting",
                "sample_id": sample_id,
                "requested": len(requests),
                "start_index": resume_index,
                "state_output": str(state_path),
            },
            ensure_ascii=False,
        )
    )

    for idx, payload in enumerate(requests, start=1):
        if idx < resume_index:
            continue
        try:
            status, body = post_json(ingest_url, payload, args.timeout_seconds)
            if status == 202:
                accepted += 1
            else:
                failed += 1
        except urllib.error.URLError as err:
            failed += 1
            body = {"error": str(err)}

        if idx % args.progress_every == 0 or idx == len(requests):
            state = {
                "sample_id": sample_id,
                "requested": len(requests),
                "start_index": resume_index,
                "last_submitted_index": idx,
                "next_index": idx + 1,
                "accepted_in_current_run": accepted,
                "failed_in_current_run": failed,
                "elapsed_seconds": round(time.time() - started_at, 3),
                "resume_command": f"python3 benchmark/locomo/ingest_locomo_sample.py --input {input_path} --start-index {idx + 1}",
                "last_error": None if failed == 0 else body,
            }
            save_state(state_path, state)
            print(
                f"sample={sample_id} last_submitted_index={idx} next_index={idx + 1} accepted={accepted} failed={failed}"
            )

        if args.pause_ms > 0:
            time.sleep(args.pause_ms / 1000)

    summary = {
        "sample_id": sample_id,
        "requested": len(requests),
        "start_index": resume_index,
        "accepted": accepted,
        "failed": failed,
        "last_submitted_index": len(requests),
        "next_index": len(requests) + 1,
        "elapsed_seconds": round(time.time() - started_at, 3),
        "resume_command": f"python3 benchmark/locomo/ingest_locomo_sample.py --input {input_path} --start-index {len(requests) + 1}",
        "state_output": str(state_path),
    }
    save_state(state_path, summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0 if failed == 0 else 2


if __name__ == "__main__":
    raise SystemExit(main())
