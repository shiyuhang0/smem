#!/usr/bin/env python3

import argparse
import json
import math
import os
import statistics
import time
import urllib.error
import urllib.request
from pathlib import Path


QA_PROMPT = """Based on the recalled memories below, answer the question with a short phrase. Use exact words from the memories whenever possible.

Recalled memories:
{context}

Question: {question}
Short answer:"""


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Run smem recall plus LLM QA benchmark for one LoCoMo sample file."
    )
    parser.add_argument("--input", required=True, help="Path to one-sample LoCoMo JSON file")
    parser.add_argument(
        "--smem-base-url",
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
        default="benchmark/locomo/out/qa",
        help="Directory to write benchmark outputs",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=int,
        default=60,
        help="HTTP timeout per request",
    )
    parser.add_argument(
        "--llm-base-url",
        default=os.environ.get("OPENAI_BASE_URL", "https://api.openai.com/v1"),
        help="OpenAI-compatible base URL",
    )
    parser.add_argument(
        "--llm-api-key",
        default=os.environ.get("OPENAI_API_KEY", ""),
        help="OpenAI-compatible API key",
    )
    parser.add_argument(
        "--llm-model",
        default=os.environ.get("OPENAI_MODEL", os.environ.get("OPENAI_CHAT_MODEL", "gpt-4.1-mini")),
        help="OpenAI-compatible chat model",
    )
    parser.add_argument(
        "--max-answer-tokens",
        type=int,
        default=64,
        help="Max tokens for generated short answer",
    )
    return parser.parse_args()


def post_json(url: str, payload: dict, timeout_seconds: int, headers: dict | None = None) -> dict:
    data = json.dumps(payload).encode("utf-8")
    request_headers = {"Content-Type": "application/json"}
    if headers:
        request_headers.update(headers)
    req = urllib.request.Request(url, data=data, headers=request_headers, method="POST")
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


def build_context(items: list[dict]) -> str:
    if not items:
        return "No relevant memory recalled."
    lines: list[str] = []
    for idx, item in enumerate(items, start=1):
        content = item.get("memory", {}).get("content", "")
        lines.append(f"[{idx}] {content}")
    return "\n".join(lines)


def call_llm(base_url: str, api_key: str, model: str, prompt: str, timeout_seconds: int, max_answer_tokens: int) -> tuple[str, dict]:
    if not api_key:
        raise SystemExit("llm api key is required; pass --llm-api-key or set OPENAI_API_KEY")

    url = base_url.rstrip("/") + "/chat/completions"
    payload = {
        "model": model,
        "messages": [
            {"role": "system", "content": "You answer questions using recalled long-term memories. Keep answers short and factual."},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0,
        "max_tokens": max_answer_tokens,
    }
    response = post_json(
        url,
        payload,
        timeout_seconds,
        headers={"Authorization": f"Bearer {api_key}"},
    )
    choices = response.get("choices", [])
    if not choices:
        raise SystemExit(f"llm response missing choices: {response}")
    message = choices[0].get("message", {})
    content = message.get("content", "")
    if isinstance(content, list):
        content = "".join(part.get("text", "") for part in content if isinstance(part, dict))
    return str(content).strip(), response.get("usage", {})


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

    recall_url = args.smem_base_url.rstrip("/") + "/api/v1/memories/recall"
    recall_latencies_ms: list[float] = []
    llm_latencies_ms: list[float] = []
    results: list[dict] = []

    for idx, qa in enumerate(qas, start=1):
        recall_payload = {
            "content": qa["question"],
            "top_k": args.top_k,
            "temperature": args.temperature,
        }
        recall_started = time.perf_counter()
        try:
            recall_response = post_json(recall_url, recall_payload, args.timeout_seconds)
        except urllib.error.URLError as err:
            raise SystemExit(f"failed to call {recall_url}: {err}")
        recall_latency_ms = round((time.perf_counter() - recall_started) * 1000, 3)
        recall_latencies_ms.append(recall_latency_ms)

        items = recall_response.get("items", [])
        context = build_context(items)
        prompt = QA_PROMPT.format(context=context, question=qa["question"])

        llm_started = time.perf_counter()
        answer, usage = call_llm(
            args.llm_base_url,
            args.llm_api_key,
            args.llm_model,
            prompt,
            args.timeout_seconds,
            args.max_answer_tokens,
        )
        llm_latency_ms = round((time.perf_counter() - llm_started) * 1000, 3)
        llm_latencies_ms.append(llm_latency_ms)

        results.append(
            {
                "index": idx,
                "question": qa["question"],
                "gold_answer": qa.get("answer"),
                "category": qa.get("category"),
                "evidence": qa.get("evidence", []),
                "prediction": answer,
                "recall_latency_ms": recall_latency_ms,
                "llm_latency_ms": llm_latency_ms,
                "retrieved_count": len(items),
                "usage": usage,
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

        print(
            f"sample={sample_id} qa={idx}/{len(qas)} recall_ms={recall_latency_ms} llm_ms={llm_latency_ms} retrieved={len(items)} prediction={answer}"
        )

    sorted_recall = sorted(recall_latencies_ms)
    sorted_llm = sorted(llm_latencies_ms)
    summary = {
        "sample_id": sample_id,
        "input": str(input_path),
        "question_count": len(qas),
        "top_k": args.top_k,
        "temperature": args.temperature,
        "llm_model": args.llm_model,
        "recall_latency_ms": {
            "mean": round(statistics.mean(recall_latencies_ms), 3) if recall_latencies_ms else 0.0,
            "p50": round(percentile(sorted_recall, 0.50), 3),
            "p95": round(percentile(sorted_recall, 0.95), 3),
            "p99": round(percentile(sorted_recall, 0.99), 3),
            "max": round(max(recall_latencies_ms), 3) if recall_latencies_ms else 0.0,
        },
        "llm_latency_ms": {
            "mean": round(statistics.mean(llm_latencies_ms), 3) if llm_latencies_ms else 0.0,
            "p50": round(percentile(sorted_llm, 0.50), 3),
            "p95": round(percentile(sorted_llm, 0.95), 3),
            "p99": round(percentile(sorted_llm, 0.99), 3),
            "max": round(max(llm_latencies_ms), 3) if llm_latencies_ms else 0.0,
        },
        "results_output": str(output_dir / f"{sample_id}_top{args.top_k}_qa_results.json"),
    }

    results_path = output_dir / f"{sample_id}_top{args.top_k}_qa_results.json"
    summary_path = output_dir / f"{sample_id}_top{args.top_k}_qa_summary.json"
    results_path.write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    summary_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
