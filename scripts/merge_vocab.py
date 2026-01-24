#!/usr/bin/env python3
import argparse
import csv
from pathlib import Path

LANG_CODE = {
    "english": "en",
    "russian": "ru",
    "dutch": "nl",
    "spanish": "es",
    "german": "de",
    "french": "fr",
}


def parse_pair(name):
    stem = name.removesuffix(".csv")
    parts = stem.split("-")
    if len(parts) != 2:
        return None
    left = LANG_CODE.get(parts[0])
    right = LANG_CODE.get(parts[1])
    if not left or not right:
        return None
    return left, right


def main():
    parser = argparse.ArgumentParser(description="Merge vocabularies into a single CSV.")
    parser.add_argument("input_dir", help="Directory containing vocabulary CSV files")
    parser.add_argument("output_csv", help="Output merged CSV path")
    args = parser.parse_args()

    input_dir = Path(args.input_dir)
    output_csv = Path(args.output_csv)

    english_to = {"ru": {}, "nl": {}, "es": {}, "de": {}, "fr": {}}
    english_order = []
    english_seen = set()

    for csv_path in sorted(input_dir.glob("*.csv")):
        pair = parse_pair(csv_path.name)
        if not pair:
            continue
        left, right = pair
        if left != "en" and right != "en":
            continue
        with csv_path.open(newline="", encoding="utf-8") as handle:
            reader = csv.reader(handle)
            for row in reader:
                if len(row) < 2:
                    continue
                left_word = row[0].strip()
                right_word = row[1].strip()
                if not left_word or not right_word:
                    continue

                if left == "en":
                    english = left_word
                    other_lang = right
                    other_word = right_word
                else:
                    english = right_word
                    other_lang = left
                    other_word = left_word

                if english not in english_seen:
                    english_order.append(english)
                    english_seen.add(english)

                if other_lang in english_to:
                    english_to[other_lang][english] = other_word

    if not english_order:
        print("No English-keyed vocabularies found.")
        return 2

    output_csv.parent.mkdir(parents=True, exist_ok=True)
    header = ["en", "ru", "nl", "es", "de", "fr"]
    missing_counts = {lang: 0 for lang in header if lang != "en"}

    with output_csv.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(header)
        for english in english_order:
            row = [english]
            for lang in header[1:]:
                value = english_to[lang].get(english, "")
                if value == "":
                    missing_counts[lang] += 1
                row.append(value)
            writer.writerow(row)

    for lang, count in missing_counts.items():
        if count:
            print(f"missing {lang}: {count}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
