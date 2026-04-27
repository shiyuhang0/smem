# LoCoMo Benchmark

## Ingest One Sample At A Time

If you want to import one sample at a time, use the split sample file directly:

```bash
python3 benchmark/locomo/ingest_locomo_sample.py \
  --input benchmark/locomo/out/samples/conv-26.json
```

This script:

1. preprocesses the sample into turn-level `content + mode` requests
2. writes them under `benchmark/locomo/out/sample_ingest/<sample_id>/`
3. ingests them directly into `smem`

Example outputs for `conv-26`:

- `benchmark/locomo/out/sample_ingest/conv-26/requests.jsonl`
- `benchmark/locomo/out/sample_ingest/conv-26/manifest.json`
- `benchmark/locomo/out/sample_ingest/conv-26/ingest_state.json`

Useful options:

```bash
python3 benchmark/locomo/ingest_locomo_sample.py \
  --input benchmark/locomo/out/samples/conv-30.json \
  --limit 100 \
  --start-index 51
```

Resume state is intentionally simple and stored in a single file:

- `benchmark/locomo/out/sample_ingest/<sample_id>/ingest_state.json`

## Split Into One File Per Sample

If you want to benchmark one sample at a time:

```bash
python3 benchmark/locomo/split_locomo_samples.py \
  --input benchmark/locomo/data/locomo10.json \
  --output-dir benchmark/locomo/out/samples
```

This writes:

- `benchmark/locomo/out/samples/conv-26.json`
- `benchmark/locomo/out/samples/conv-30.json`
- ...
- `benchmark/locomo/out/samples/manifest.json`

## Run Recall Benchmark For One Sample

After one sample has been fully ingested into `smem`, benchmark recall for that sample only:

```bash
python3 benchmark/locomo/benchmark_recall_sample.py \
  --input benchmark/locomo/out/samples/conv-26.json \
  --top-k 5
```

Outputs:

- `benchmark/locomo/out/recall/conv-26_top5_results.json`
- `benchmark/locomo/out/recall/conv-26_top5_summary.json`

Useful options:

```bash
python3 benchmark/locomo/benchmark_recall_sample.py \
  --input benchmark/locomo/out/samples/conv-30.json \
  --top-k 10 \
  --limit 20
```

## Run QA Benchmark For One Sample

If you want recall results to be fed into an LLM and produce final answers:

```bash
python3 benchmark/locomo/benchmark_qa_sample.py \
  --input benchmark/locomo/out/samples/conv-26.json \
  --top-k 5 \
  --llm-api-key "$OPENAI_API_KEY" \
  --llm-model gpt-4.1-mini
```

This script uses only `qa.question` as the recall query, then sends the recalled memory contents to an OpenAI-compatible chat model.

Outputs:

- `benchmark/locomo/out/qa/conv-26_top5_qa_results.json`
- `benchmark/locomo/out/qa/conv-26_top5_qa_summary.json`

## Run Recall Benchmark For One Sample

If you only want recall results without calling an LLM:

```bash
python3 benchmark/locomo/benchmark_recall_sample.py \
  --input benchmark/locomo/out/samples/conv-26.json \
  --top-k 5
```

By default, benchmark scripts wait `5s` between questions to reduce provider-side rate limiting. Override with `--pause-seconds` if needed.

Outputs:

- `benchmark/locomo/out/recall/conv-26_top5_results.json`
- `benchmark/locomo/out/recall/conv-26_top5_summary.json`
