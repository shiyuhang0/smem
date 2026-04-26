#!/usr/bin/env python3

import argparse
import json
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Split locomo10.json into one file per sample."
    )
    parser.add_argument("--input", required=True, help="Path to locomo10.json")
    parser.add_argument(
        "--output-dir",
        required=True,
        help="Directory to write per-sample JSON files",
    )
    return parser.parse_args()


def count_conversation(sample: dict) -> tuple[int, int]:
    sessions = 0
    turns = 0
    conversation = sample["conversation"]
    for key, value in conversation.items():
        if key.startswith("session_") and not key.endswith("_date_time"):
            sessions += 1
            turns += len(value)
    return sessions, turns


def main() -> None:
    args = parse_args()
    input_path = Path(args.input)
    output_dir = Path(args.output_dir)

    samples = json.loads(input_path.read_text(encoding="utf-8"))
    output_dir.mkdir(parents=True, exist_ok=True)

    manifest: list[dict] = []
    for sample in samples:
        sample_id = sample["sample_id"]
        sessions, turns = count_conversation(sample)
        output_path = output_dir / f"{sample_id}.json"
        output_path.write_text(
            json.dumps([sample], ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        manifest.append(
            {
                "sample_id": sample_id,
                "output": str(output_path),
                "sessions": sessions,
                "turns": turns,
                "qas": len(sample.get("qa", [])),
            }
        )

    manifest_path = output_dir / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(
        json.dumps(
            {
                "output_dir": str(output_dir),
                "sample_count": len(manifest),
                "manifest": str(manifest_path),
            },
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
