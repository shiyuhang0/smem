#!/usr/bin/env python3

import argparse
import json
import math
import statistics
import time
import urllib.error
import urllib.request
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run smem recall benchmark for one LoCoMo sample file."
    )
    parser.add_argument("--input", required=True, help="Path to one-sample LoCoMo JSON file")
    parser.add_argument(
        "--base-url",
        default="http://localhost:8080",
        help="smem server base URL",
    )
    parser.add_argument("--top-k", type=int, default=5, help="Recall top_k")
    parser.add_argument("--temperature", type=float, default=1.0, help="Recall temperature")
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="Only benchmark the first N questions; 0 means all",
    )
    parser.add_argument(
        "--output-dir",
        default="benchmark/locomo/out/recall",
        help="Directory to write benchmark outputs",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=int,
        default=60,
        help="HTTP timeout per recall request",
    )
    parser.add_argument(
        "--pause-seconds",
        type=float,
        default=5.0,
        help="Pause between recall requests in seconds",
    )
    parser.add_argument(
        "--start-index",
        type=int,
        default=1,
        help="1-based QA index to start from",
    )
    return parser.parse_args()


def post_json(url: str, payload: dict, timeout_seconds: int) -> dict:
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:
        body = resp.read().decode("utf-8")
        return json.loads(body) if body else {}


def percentile(values: list[float], p: float) -> float:
    if not values:
        return 0.0
    if len(values) == 1:
        return values[0]
    rank = (len(values) - 1) * p
    lower = math.floor(rank)
    upper = math.ceil(rank)
    if lower == upper:
        return values[lower]
    weight = rank - lower
    return values[lower] * (1 - weight) + values[upper] * weight


def main() -> None:
    args = parse_args()
    input_path = Path(args.input)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    samples = json.loads(input_path.read_text(encoding="utf-8"))
    if len(samples) != 1:
        raise SystemExit("input must contain exactly one sample")

    sample = samples[0]
    sample_id = sample["sample_id"]
    qas = sample.get("qa", [])
    if args.limit > 0:
        qas = qas[: args.limit]
    if args.start_index < 1:
        args.start_index = 1

    recall_url = args.base_url.rstrip("/") + "/api/v1/memories/recall"
    latencies_ms: list[float] = []
    results: list[dict] = []
    errors: list[dict] = []

    results_path = output_dir / f"{sample_id}_top{args.top_k}_results.json"
    summary_path = output_dir / f"{sample_id}_top{args.top_k}_summary.json"

    def write_progress() -> None:
        sorted_latencies = sorted(latencies_ms)
        summary = {
            "sample_id": sample_id,
            "input": str(input_path),
            "question_count": len(qas),
            "start_index": args.start_index,
            "processed": len(results) + len(errors),
            "successful": len(results),
            "failed": len(errors),
            "top_k": args.top_k,
            "temperature": args.temperature,
            "latency_ms": {
                "mean": round(statistics.mean(latencies_ms), 3) if latencies_ms else 0.0,
                "p50": round(percentile(sorted_latencies, 0.50), 3),
                "p95": round(percentile(sorted_latencies, 0.95), 3),
                "p99": round(percentile(sorted_latencies, 0.99), 3),
                "max": round(max(latencies_ms), 3) if latencies_ms else 0.0,
            },
            "results_output": str(results_path),
            "errors": errors,
        }
        results_path.write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        summary_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        return summary

    for idx, qa in enumerate(qas, start=1):
        if idx < args.start_index:
            continue
        payload = {
            "content": qa["question"],
            "top_k": args.top_k,
            "temperature": args.temperature,
        }
        started_at = time.perf_counter()
        try:
            response = post_json(recall_url, payload, args.timeout_seconds)
        except urllib.error.HTTPError as err:
            body = err.read().decode("utf-8", errors="replace")
            error = {
                "index": idx,
                "question": qa["question"],
                "status": err.code,
                "body": body,
            }
            errors.append(error)
            print(f"sample={sample_id} qa={idx}/{len(qas)} error_status={err.code}")
            write_progress()
            continue
        except urllib.error.URLError as err:
            error = {
                "index": idx,
                "question": qa["question"],
                "status": "url_error",
                "body": str(err),
            }
            errors.append(error)
            print(f"sample={sample_id} qa={idx}/{len(qas)} url_error={err}")
            write_progress()
            continue
        latency_ms = round((time.perf_counter() - started_at) * 1000, 3)
        latencies_ms.append(latency_ms)

        items = response.get("items", [])
        results.append(
            {
                "index": idx,
                "question": qa["question"],
                "answer": qa.get("answer"),
                "category": qa.get("category"),
                "evidence": qa.get("evidence", []),
                "latency_ms": latency_ms,
                "retrieved_count": len(items),
                "retrieved": [
                    {
                        "content": item.get("memory", {}).get("content"),
                        "score": item.get("score"),
                        "reason": item.get("reason"),
                    }
                    for item in items
                ],
            }
        )
        print(f"sample={sample_id} qa={idx}/{len(qas)} latency_ms={latency_ms} retrieved={len(items)}")
        write_progress()
        if idx < len(qas) and args.pause_seconds > 0:
            time.sleep(args.pause_seconds)

    summary = write_progress()
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
