#!/usr/bin/env python3
import argparse
import csv
import json
import random
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

DEEPL_FREE_ENDPOINT = "https://api-free.deepl.com/v2/translate"
REQUEST_DELAY_SECONDS = 0.3
MAX_RETRIES = 10
TIMEOUT_SECONDS = 30


def translate(text, source, target, cache, auth_key, endpoint):
    text = text.strip()
    if not text:
        return ""
    cached = cache.get(text)
    if cached is not None:
        return cached

    payload = {
        "text": text,
        "source_lang": source,
        "target_lang": target,
    }
    data = urllib.parse.urlencode(payload).encode("utf-8")
    headers = {
        "Authorization": f"DeepL-Auth-Key {auth_key}",
        "Content-Type": "application/x-www-form-urlencoded",
    }

    last_error = None
    for attempt in range(MAX_RETRIES):
        try:
            req = urllib.request.Request(endpoint, data=data, headers=headers, method="POST")
            with urllib.request.urlopen(req, timeout=TIMEOUT_SECONDS) as response:
                body = response.read()
            result = json.loads(body)
            translations = result.get("translations", [])
            if not translations:
                raise RuntimeError(f"empty translation for {source}->{target}: {text}")
            translated = translations[0].get("text", "")
            if translated == "":
                raise RuntimeError(f"empty translation for {source}->{target}: {text}")
            cache[text] = translated
            time.sleep(REQUEST_DELAY_SECONDS)
            return translated
        except urllib.error.HTTPError as exc:
            last_error = exc
            if exc.code == 456:
                raise RuntimeError("DeepL quota exceeded") from exc
            if exc.code not in (429, 500, 502, 503, 504):
                raise
        except urllib.error.URLError as exc:
            last_error = exc

        wait = (2 ** attempt) + random.random()
        print(f"retrying {source}->{target} after error: {last_error}", file=sys.stderr)
        time.sleep(wait)

    raise RuntimeError(f"failed translation for {source}->{target}: {text}") from last_error


def main():
    parser = argparse.ArgumentParser(description="Translate vocabularies using DeepL.")
    parser.add_argument("input_csv", help="CSV file with Dutch-English pairs")
    parser.add_argument("output_dir", help="Directory for translated CSV files")
    parser.add_argument("--deepl-key", required=True, help="DeepL API key (Free or Pro)")
    parser.add_argument("--endpoint", default=DEEPL_FREE_ENDPOINT, help="DeepL API endpoint")
    args = parser.parse_args()

    input_path = Path(args.input_csv)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    output_paths = {
        "dutch_russian": output_dir / "dutch-russian.csv",
        "english_russian": output_dir / "english-russian.csv",
        "spanish_english": output_dir / "spanish-english.csv",
        "spanish_russian": output_dir / "spanish-russian.csv",
        "german_english": output_dir / "german-english.csv",
        "german_russian": output_dir / "german-russian.csv",
        "french_english": output_dir / "french-english.csv",
        "french_russian": output_dir / "french-russian.csv",
    }

    caches = {
        "nl_ru": {},
        "en_ru": {},
        "en_es": {},
        "en_de": {},
        "en_fr": {},
    }

    with input_path.open(newline="", encoding="utf-8") as handle, \
        output_paths["dutch_russian"].open("w", newline="", encoding="utf-8") as dutch_ru, \
        output_paths["english_russian"].open("w", newline="", encoding="utf-8") as english_ru, \
        output_paths["spanish_english"].open("w", newline="", encoding="utf-8") as spanish_en, \
        output_paths["spanish_russian"].open("w", newline="", encoding="utf-8") as spanish_ru, \
        output_paths["german_english"].open("w", newline="", encoding="utf-8") as german_en, \
        output_paths["german_russian"].open("w", newline="", encoding="utf-8") as german_ru, \
        output_paths["french_english"].open("w", newline="", encoding="utf-8") as french_en, \
        output_paths["french_russian"].open("w", newline="", encoding="utf-8") as french_ru:
        reader = csv.reader(handle)
        dutch_ru_writer = csv.writer(dutch_ru)
        english_ru_writer = csv.writer(english_ru)
        spanish_en_writer = csv.writer(spanish_en)
        spanish_ru_writer = csv.writer(spanish_ru)
        german_en_writer = csv.writer(german_en)
        german_ru_writer = csv.writer(german_ru)
        french_en_writer = csv.writer(french_en)
        french_ru_writer = csv.writer(french_ru)

        for idx, row in enumerate(reader, start=1):
            if len(row) < 2:
                continue
            dutch = row[0].strip()
            english = row[1].strip()
            if not dutch or not english:
                continue

            russian_from_dutch = translate(dutch, "NL", "RU", caches["nl_ru"], args.deepl_key, args.endpoint)
            russian_from_english = translate(english, "EN", "RU", caches["en_ru"], args.deepl_key, args.endpoint)
            spanish_from_english = translate(english, "EN", "ES", caches["en_es"], args.deepl_key, args.endpoint)
            german_from_english = translate(english, "EN", "DE", caches["en_de"], args.deepl_key, args.endpoint)
            french_from_english = translate(english, "EN", "FR", caches["en_fr"], args.deepl_key, args.endpoint)

            dutch_ru_writer.writerow([dutch, russian_from_dutch])
            english_ru_writer.writerow([english, russian_from_english])
            spanish_en_writer.writerow([spanish_from_english, english])
            spanish_ru_writer.writerow([spanish_from_english, russian_from_english])
            german_en_writer.writerow([german_from_english, english])
            german_ru_writer.writerow([german_from_english, russian_from_english])
            french_en_writer.writerow([french_from_english, english])
            french_ru_writer.writerow([french_from_english, russian_from_english])

            if idx % 50 == 0:
                print(f"translated {idx} rows")

    print("done")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
