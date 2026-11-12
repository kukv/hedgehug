# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repository uses **RDRAAgent** (v0.6) to rebuild the hedgehug requirements with the **RDRA (Relationship Driven Requirement Analysis)** methodology. `初期要望.txt` holds the hedgehug initial requirements; the earlier hand-written design documents are under `docs/`.

RDRAAgent is a Node.js CLI (`node menu.js`) that runs Claude Code (`claude -p`) in parallel to generate RDRA definitions and specification drafts as TSV / Markdown / JSON files.

- No npm dependencies (pure Node.js built-in modules). Do not add `package.json` dependencies.
- Text-based data only (TSV, Markdown, JSON). No database.
- Supported OS: macOS and Windows. Menus 11, 12 and 22 use the clipboard and browser and return "not supported" on Linux.

## Architecture

### Entry point and menu

- `menu.js` — readline menu. Dispatches to `RDRA_Knowledge/helper_tools/menuAction.js`.
- Menu options:
  - `1` run the first Phase (1–4) whose output files are missing; if Phase4 is complete, run Phase5
  - `2` run all missing Phases through Phase5
  - `11` build `1_RDRA/関連データ.txt`, copy it to the clipboard and open RDRAGraph
  - `12` build `1_RDRA/ZeroOne.txt`, copy it and open the Google Spreadsheet template
  - `21` generate the specification into `2_RDRASpec/` (requires `1_RDRA/関連データ.txt`)
  - `22` start `web_tool/bucActorUI.js` on port 3002 and open it (requires menu 21 output)
  - `0` exit (stops the server), `99` delete generated files

### LLM execution

- `helper_tools/parallelRun/parallel-runner.js` runs one `claude -p` per prompt file concurrently, via `node-file-runner-V3.js`.
- The CLI, model and arguments come from `モデル設定.json` (`default.provider`, `providers.<name>`). The claude default is:
  `claude -p --output-format text --permission-mode auto --setting-sources= --model claude-sonnet-5-5 --append-system-prompt-file <file>`
  - `--setting-sources=` keeps user/project settings and this `CLAUDE.md` out of the generation runs.
- The prompt body is sent on stdin. Shared context for a Phase (`RDRA.md`, `初期要望.txt`, previous Phase outputs) is assembled by `helper_tools/tokenOptimization/generateSystemPrompt.js` into `RDRA_Knowledge/system_prompts/` and passed with `--append-system-prompt-file`.
- Timeout: 600000 ms per prompt (default in `parallel-runner.js`; menu 21 also passes it explicitly).

### Configuration

- `helper_tools/settings/rdraConfig.js` — prompt lists per Phase, expected output files per Phase (used to decide which Phase to run), and shared-context file lists.

### Data flow

```
初期要望.txt
  → Phase1–4 prompts (RDRA_Knowledge/0_RDRAZeroOne/phaseN/*.md)
  → 0_RDRAZeroOne/phase1–4/  (TSV, 初期要望分析.md)
  → Phase5: rdraFileCopy.js + phase5/システム概要生成.md + makeGraphData.js
  → 1_RDRA/  (TSV, システム概要.json, 関連データ.txt)
  → menu 21: RDRA_Knowledge/2_RDRASpec/phase1/*.md, then phase2/*.md
  → 2_RDRASpec/  (phase1/*.json, 論理データモデル.md, ビジネスルール.md, 画面照会.json)
```

### Knowledge base

- `RDRA_Knowledge/1_RDRA/RDRA.md` — RDRA model definitions
- `RDRA_Knowledge/1_RDRA/RDRAGraph.md`, `RDRASheet.md` — output formats
- `RDRA_Knowledge/0_RDRAZeroOne/`, `RDRA_Knowledge/2_RDRASpec/` — generation prompts

## Rules for generated files

From `.claude/rules/rdrarules.mdc` / `.cursor/rules/rdrarules.mdc`:

- Reuse an existing folder instead of creating a new one; overwrite files with the same name.
- TSV: UTF-8, tab-separated, first row is the header.
- Programs are JavaScript / HTML (CSS inside HTML) only.
- Do not create template or placeholder files for outputs; let the generation write the full content.

## Common tasks

- Change LLM behavior: edit the prompt files under `RDRA_Knowledge/0_RDRAZeroOne/` or `RDRA_Knowledge/2_RDRASpec/`.
- Change model or CLI: edit `モデル設定.json`.
- Re-run from a Phase: delete that Phase's output folder and every later one (e.g. `0_RDRAZeroOne/phase3/` and `phase4/`), then run menu 2. Phases whose files are complete are skipped, so a kept later Phase would feed stale output into Phase5.
- Try a sample: copy `Samples/<name>/初期要望.txt` to the root `初期要望.txt`.

## External resources

- RDRAGraph: https://vsa.co.jp/rdratool/graph/v0.94/
- Google Spreadsheet template: https://docs.google.com/spreadsheets/d/1h7J70l6DyXcuG0FKYqIpXXfdvsaqjdVFwc6jQXSh9fM/
