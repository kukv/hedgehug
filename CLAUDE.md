# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

RDRAAgent is a Node.js-based system that implements the **RDRA (Relationship Driven Requirement Analysis)** methodology for systematic software requirements definition. It guides users through a 4-phase process to progressively refine business requirements from initial concepts into detailed system specifications.

**Key Characteristics:**
- No external dependencies (pure Node.js with native modules)
- Text/TSV-based data format (no databases)
- LLM-integrated workflow (Claude) for AI-assisted analysis
- Multi-platform support (Windows, macOS)
- Modular phase-based architecture

## Architecture Overview

### Core Components

1. **menu.js** (Entry Point)
   - Interactive CLI menu system using Node.js `readline`
   - Orchestrates all operations and phase management
   - Manages concurrent server processes (UI editors)
   - Cross-platform command execution (Windows cmd/PowerShell, macOS Terminal)

2. **Three Main Processing Streams**

   **Stream 1: ZeroOne (0_RDRAZeroOne/** - Requirements Definition)
   - Phase1: Initial element extraction (actors, systems, policies, workflows, information, states)
   - Phase2: Element refinement and detailed conditions
   - Phase3: Context mapping and Business Use Case (BUC) definition
   - Phase4: Relationship model transformation
   - Phase5: Consolidation into unified RDRA format (1_RDRA/)

   **Stream 2: RDRA (1_RDRA/** - Requirements Visualization & Management)
   - Consolidates all phase outputs
   - Generates relationship data for RDRAGraph visualization
   - Supports Google Spreadsheet export
   - Validates definition consistency

   **Stream 3: RDRASpec (2_RDRASpec/** - Specification Generation)
   - Logical data model (TSV/Markdown)
   - UI definitions (JSON format)
   - Business rules documentation
   - Specification validation

3. **Helper Tools** (RDRA_Knowledge/helper_tools/**)
   - `makeGraphData.js` - Generates relationship data for RDRAGraph visualization
   - `makeZeroOneData.js` - Prepares data for Google Spreadsheet export
   - `copyToClipboard.js` - Copy data to the clipboard
   - `convertUI2ActorUI.js` - Transforms UI definitions by actor role
   - `deleteFiles.js` - Cleanup utility
   - `rdraFileCopy.js` - Consolidates outputs from phases
   - `drawioExtractor.js` - Processes draw.io diagrams
   - `makeActorUI.js` - Actor-specific UI generation
   - Web servers (`ui_server.js`, `actorUI_server.js`) - HTTP servers for browser-based editing (ports 3000, 3001)

4. **Knowledge Base** (RDRA_Knowledge/**)
   - **0_ZeroOne指示.md** - Phase-by-phase requirements extraction instructions
   - **1_RDRA指示.md** - (deprecated) moved under `_no_use/deprecated_menu_13_14_22/`
   - **2_Spec指示.md** - Specification creation instructions (Phase1 via menu 21; Phase2 is deprecated)
   - **0_RDRAZeroOne/**, **1_RDRA/**, **2_RDRASpec/** - Domain knowledge documentation

### Data Flow Model

```
初期要望.txt (Initial Requirements)
         ↓
    menu.js selector
         ↓
    ├─→ Phase1-5 (LLM-driven via Claude)
    │    (RDRA_Knowledge/0_ZeroOne指示.md)
    │         ↓
    │    0_RDRAZeroOne/phase1~4/ (TSV outputs)
    │         ↓
    │    1_RDRA/ (Consolidated)
    │
    ├─→ Visualization
    │    makeGraphData.js → RDRAGraph (external tool)
    │
    ├─→ Validation
    │    _no_use/deprecated_menu_13_14_22/RDRA_Knowledge/1_RDRA指示.md
    │    (checks against 妥当性検証環境.csv)
    │
    └─→ Specification
         RDRA_Knowledge/2_Spec指示.md
              ↓
         2_RDRASpec/
         ├─ 論理データモデル.md (logical data)
         ├─ 画面照会.json (screen definitions)
         └─ ビジネスルール.md (rules)
```

## Cursor/Claude Rules

Key rules from `.cursor/rules/rdrarules`:

1. **Folder Management**: Check if folders exist before creating; reuse existing directories
2. **File Formats**:
   - TSV files use `、` as tab delimiter (convert to actual tabs on output)
   - UTF-8 encoding required
   - TSV files always have header row with column names
3. **Programming**: JavaScript only (HTML/CSS embedded in HTML)
4. **UI Library**: shadcn UI components used in web-based editors
5. **Execution Flow**:
   - "return" = function exit with message
   - "StepN:" = sequential execution
   - "start process" = command/browser/JavaScript execution
   - "Goal:" indicates function purpose
6. **File Generation**: Do NOT create template/placeholder files; let the LLM generate full content

## Common Development Tasks

### Starting the Application
```bash
node menu.js
```
Opens interactive menu with 9 main options.

### Single File Operations
Within menu, operations like "Phase 1" or "Create Spec" trigger LLM execution via:
- **Windows**: `RDRA_Knowledge/helper_tools/command/commandRun.bat`
- **macOS**: `RDRA_Knowledge/helper_tools/command/commandRun.sh`

These scripts open new terminal windows and run Claude Code with specified instructions.

### Testing/Development Workflow
1. Modify `RDRA_Knowledge/*指示.md` files for LLM behavior changes
2. Update `menu.js` for UI/menu changes
3. Update helper tools in `RDRA_Knowledge/helper_tools/` for data transformation
4. Sample projects in `Samples/` can be used for quick testing:
   ```bash
   copy "Samples/図書館システム/初期要望.txt" "初期要望.txt"
   node menu.js
   ```

### Validation/Testing Data
- `妥当性検証環境.csv` - Define test scenarios for requirement validation
- Structure: actor roles, people counts, constraints per location
- Used by Phase2 validation logic (menu option 14)

## TSV File Format Rules

- **Delimiter**: `、` character in source code becomes actual TAB when written to files
- **First Row**: Must be headers (column names)
- **Encoding**: UTF-8 always
- **Output Location**: Phase outputs go to `0_RDRAZeroOne/phase#/`, final outputs to `1_RDRA/` or `2_RDRASpec/`

Example header transformation:
```javascript
"Column1、Column2、Column3"  →  "Column1\tColumn2\tColumn3"
```

## LLM Integration Points

The system uses Claude Code (via Terminal execution) for:

1. **Phase1-4 Content Generation** (`menu.js:1,2` - options 1,2)
   - Reads: `RDRA_Knowledge/0_ZeroOne指示.md`
   - Generates: TSV files in `0_RDRAZeroOne/phase#/`

2. **RDRA Explanation & Validation** (options 13,14 are deprecated/removed from menu)
   - Reads: `_no_use/deprecated_menu_13_14_22/RDRA_Knowledge/1_RDRA指示.md` (deprecated)
   - Validates: Against `妥当性検証環境.csv`

3. **Specification Creation** (`menu.js:21` - option 21; option 22 is deprecated/removed from menu)
   - Reads: `RDRA_Knowledge/2_Spec指示.md`
   - Generates: Data models, UI definitions, business rules

**Important**: LLM instructions include file path prefixes (e.g., "パス「RDRA_Knowledge/0_ZeroOne指示.md」") and notes about where files are located relative to working directory.

## Platform-Specific Details

### Windows
- LLM terminals: `cmd.exe` invoked via `start cmd`
- Path handling: Forward slashes converted to backslashes for PowerShell
- Clipboard: PowerShell `Set-Clipboard` with UTF-8
- Browser: PowerShell `Start-Process`

### macOS
- LLM terminals: `Terminal.app` via `osascript`
- Clipboard: `pbcopy`
- Browser: `open` command

### Port Management
- **Port 3000**: UI editor server (option 23)
- **Port 3001**: Actor-specific UI server (option 24)
- Reuses existing processes if already running; checks for EADDRINUSE errors

## Important Implementation Notes

1. **No npm Dependencies**: This is intentional. Avoid adding `package.json` dependencies.

2. **File Overwriting**: Menu.js automatically overwrites existing files with same names during regeneration.

3. **Phase Interdependencies**:
   - Phase 1 outputs become input for Phase 2 user review
   - Phase 3,4 outputs are consolidated into 1_RDRA folder
   - Spec generation requires complete 1_RDRA folder

4. **Process Management**:
   - Global variables store spawned processes: `global.serverProcess`, `global.actorScreenServer`
   - Graceful shutdown kills all processes on menu exit (option 0)

5. **File Organization**:
   - Input: `初期要望.txt`, `妥当性検証環境.csv`
   - Output phases: `0_RDRAZeroOne/`, `1_RDRA/`, `2_RDRASpec/`
   - Knowledge/tools: `RDRA_Knowledge/`
   - Samples: `Samples/`

## External Resources

- **RDRAGraph Visualization**: https://vsa.co.jp/rdratool/graph/v0.94/
- **Google Spreadsheet Template**: https://docs.google.com/spreadsheets/d/1h7J70l6DyXcuG0FKYqIpXXfdvsaqjdVFwc6jQXSh9fM/
- **RDRA Methodology**: See `RDRA_Knowledge/1_RDRA/RDRA.md`

## Summary of Key Files

| File | Purpose |
|------|---------|
| menu.js | Main CLI orchestrator |
| RDRA_Knowledge/0_ZeroOne指示.md | Requirements generation instructions |
| _no_use/deprecated_menu_13_14_22/RDRA_Knowledge/1_RDRA指示.md | (deprecated) RDRA validation/explanation |
| RDRA_Knowledge/2_Spec指示.md | Specification generation |
| 初期要望.txt | User input: initial requirements |
| 妥当性検証環境.csv | User input: validation scenarios |
| Samples/ | Reference project templates |
| helper_tools/ | Data transformation utilities |
