# Compliance Manager

> **Repo name note:** this repository is pending a rename. Despite the current name (`Regex-Engine`), it is **not** a standalone regular-expression engine — it contains a desktop data-discovery / PII-compliance scanning application in which regex rule matching is one component among many.

A desktop application (Go + [Wails v2](https://wails.io), window title "Compliance Manager") that scans a configured directory tree for PII and compliance violations. It extracts text from documents, matches it against rule definitions, scores risk, and stores results in a local SQLite database, which the UI presents as a browsable file hierarchy with per-node risk scores and data-lineage edges (e.g. "CopyOf" relations between files).

## Architecture

Three cooperating parts live in this monorepo:

**1. Go desktop app** (repo root — module `Regex`, Go 1.24) — the Wails v2 shell plus the scanning engine:

| Area | Packages |
|---|---|
| Scan orchestration | `core/`, `pipeline/`, `dispatcher/` — scan manager (start / stop / pause / resume), progress streamed to the UI via Wails events |
| Walking & extraction | `scanner/`, `extractor/`, `sorter/`, `searcher/`, `checksum/` — directory walking, text extraction and match search; PDF / DOCX / XLSX support via `ledongthuc/pdf`, `nguyenthenguyen/docx`, `xuri/excelize` |
| Rule matching | `engine/`, `rules/` — rule evaluation on extracted text (`grafana/regexp`), rule definitions in `compliance_rules/` |
| OCR | `ocr/` — OCR hand-off for scanned content, toggleable from the UI |
| Persistence | `database/` — SQLite (`mattn/go-sqlite3`): hierarchy nodes, PII tags with risk scores, node relations |
| Supporting | `lineage/`, `events/`, `connectors/`, `governor/` (CPU cap from config), `stats/` (`gopsutil`), `analytics/`, `logger/`, `config/`, `types/` |
| ML bridge | `client/ml_client.go` — client for the Python ML engine (gRPC / protobuf dependencies in `go.mod`) |

**2. `ml-engine-core/`** — Python ML validation service ("Hexa-Core"): a 6-layer pipeline (layout analysis → NER → deterministic validators → adversarial filter → LLM semantic judge → confidence synthesis) for high-precision validation of PII findings. See [`ml-engine-core/README.md`](ml-engine-core/README.md).

**3. `ocr_py/`** — Python OCR package.

**Frontend** (`frontend/`): plain JavaScript (no framework) bundled with Vite; `frontend/wailsjs/` holds the generated Go↔JS bindings. Built assets are embedded into the binary from `frontend/dist` at compile time.

## Running

Prerequisites: Go 1.24+, the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation), Node.js (frontend build), and Python 3 for the ML / OCR services. Development so far has targeted Windows (see `app.manifest`; `config.json` uses a Windows scan root).

```bash
# desktop app — dev mode / production build
wails dev
wails build
```

ML validation service (optional, used for high-precision PII validation):

```bash
cd ml-engine-core
pip install -r requirements.txt
python -m api.main        # serves on http://localhost:8000
```

Configuration is in `config.json`: scan root (`root_dir`), rules directory, worker count, CPU percentage cap, log level and buffer sizes. The app refuses to start a scan if the rules directory (default `./compliance_rules`) is missing.

## License

No license file yet — all rights reserved until one is added.
