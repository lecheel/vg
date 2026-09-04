# vgrep

A blazing-fast, project-aware interactive CLI search tool built on top of [ripgrep (`rg`)](https://github.com/BurntSushi/ripgrep).

`vgrep` bridges the gap between searching code and editing it by providing **Vim-motion navigation**, **scoped project history recall**, **on-the-fly ripgrep queries (`n`)**, **in-app & external find-and-replace**, and direct session integration with editors like **`wig`**, **`nvim`**, and **`vim`**.

`vgrep` bridges the gap between searching code and editing it by providing **Vim-motion navigation**, **scoped project history recall**, **on-the-fly ripgrep queries (`n`)**, **in-app & external find-and-replace**, and direct session integration with editors like **`wig`**, **`nvim`**, and **`vim`**.

---

## Problem Statement

Running searches directly from terminal shells (`bash`, `zsh`) often turns into a frustrating battle with quote and space parsing:

- **Eaten Quotes (`'` and `"`)**: Shell interpreters strip or expand single and double quotes before passing arguments to commands. Searching for code snippets with quotes (e.g., `{"key": "value"}` or `'foo'`) requires tedious multi-layer escaping (`\"`, `\'`).
- **Word-Splitting on Spaces**: Searching for phrases or expressions containing spaces (e.g., `const x = "bar"`) often leads to unintentional word-splitting unless wrapped in nested quotes.
- **Accidental Shell Expansions**: Characters like `$`, `!`, `\`, and `` ` `` trigger shell variable or history expansions unless painstakingly escaped.

### How `vgrep` Solves This

- 💡 **Direct In-TUI Input (`n`)**: Press `n` in the TUI to open an interactive prompt that reads in raw terminal mode. You can freely type spaces, single quotes (`'`), double quotes (`"`), backslashes, and regex characters without worrying about shell scripts or command-line parsers eating them.
- 🔤 **Instant Case Sensitivity Toggle (`Alt+i`)**: Press `Alt+i` while typing in the `n` prompt or in normal mode to switch between case-sensitive and case-insensitive (`-i`) search instantly.
- 🔲 **Literal Mode Toggle (`F`)**: Toggle fixed strings mode (`-F`) on the fly to search symbols like `[`, `]`, `(`, `)`, and `.` literally without regex escaping.
- 🧠 **Frictionless History Recall**: Revisit past searches containing quotes or spaces directly from the project history picker without re-typing or re-escaping them in your shell.
- 🔁 **Continuous Workflow**: Stay inside your interactive search session rather than jumping back and forth to a shell prompt.

---

## Features

- ⚡ **Powered by `ripgrep`**: Instant search with automatic `.gitignore` parsing, pruning of build/target directories, and multi-threaded traversal.
- 🎯 **Vim-Motion TUI**: Navigate search results using relative line numbers, counts (e.g., `3j`, `5k`), file-aware jumps (`J`/`K`), live filtering (`/`), and return right back to your search list after `:q` in your editor.
- 🔍 **New Search On-the-Fly (`n`)**: Press `n` anytime in the TUI to open an interactive prompt and run a new ripgrep query without exiting.
- 🔲 **Literal Mode (`F` / `-F`)**: Toggle between regex and literal string matching on the fly.
- ✏️ **Built-in Find & Replace (`R` / `Tab`)**: Interactive in-place find and replace with real-time substitution preview, match exclusion toggles (`SPC`, `a`), and batch file updates.
- 🔁 **External Replacer (`r` with `rgr`)**: Press `r` in the TUI to launch [repgrep / `rgr`](https://github.com/acheronfail/repgrep) (automatically hidden if `rgr` is not installed).
- 📍 **Exact Column Positioning**: Jumps directly to the matched word and character column in editors like `wig`, `nvim`, `vim`, `helix`, `vscode`, `nano`, and `emacs`.
- 🩺 **Environment Health Check (`--health`)**: Instantly inspect installed tooling (`rg`, `fzf`, `rgr`, `$EDITOR`, config editor) and configuration paths with clean `~` path abbreviation.
- ⚙️ **Configurable (`~/.config/vgrep/config.toml`)**: Custom default editor, session cache path, and default literal search options.
- 🔍 **New Search On-the-Fly (`n`)**: Press `n` anytime in the TUI to open an interactive prompt and run a new ripgrep query without exiting.
- ✏️ **Built-in Find & Replace (`R` / `Tab`)**: Interactive in-place find and replace with real-time diff preview, match exclusions, and batch file updates.
- 🔁 **External Replacer (`r` with `rgr`)**: Press `r` in the TUI to launch [repgrep / `rgr`](https://github.com/acheronfail/repgrep) (automatically hidden if `rgr` is not installed).
- 🩺 **Environment Health Check (`--health`)**: Instantly inspect installed tooling (`rg`, `fzf`, `rgr`, `$EDITOR`) and configuration paths with clean `~` path abbreviation.
- 🧠 **Project-Scoped History**: Running `vgrep` without arguments remembers and recalls past search patterns specific to the current Git repository or workspace.
- 📂 **Auto-Detection & Quick Shorthands**:
  - Automatically identifies projects (`go.mod`, `Cargo.toml`, `pyproject.toml`, etc.).
  - Converts shorthand patterns (e.g., `myHandler_fn` $\rightarrow$ `func myHandler` or `fn myHandler`).
- ⚡ **Auto-Jump on Single Match**: Directly opens the editor if the search yields exactly one result.
- 🔄 **`wig` Quickfix Session Synchronization**: Automatically exports search matches to `~/.config/wig/rg_search.json` for editor session sharing.
- 📜 **Review Mode (`-v` / `--view`)**: Revisit and interact with your previous search session without re-scanning the codebase.

---

## Installation

### Prerequisites
- [ripgrep (`rg`)](https://github.com/BurntSushi/ripgrep) (required)
- [repgrep (`rgr`)](https://github.com/acheronfail/repgrep) (optional, enables `r` for advanced regex find & replace)
- [fzf](https://github.com/junegunn/fzf) (optional, for fuzzy history selection)
- An editor like `wig`, `nvim`, or `vim`

### Build from Source
```bash
git clone https://github.com/your-username/vgrep.git
cd vgrep
go build -o vgrep main.go
sudo mv vgrep /usr/local/bin/
```

---

## Usage

### 1. Basic Search
Search across the entire project repository:
```bash
vgrep SearchPattern
```
*If only **one match** is found, `vgrep` jumps straight into `$EDITOR +<line>`.*

### 2. Language-Specific Filtering
Filter matches by file extensions:
```bash
vgrep --go MyStruct      # Go files (*.go)
vgrep --rs MyTrait       # Rust files (*.rs)
vgrep --py def           # Python files (*.py)
vgrep --cc MyClass       # C/C++ files (*.c, *.cpp, *.h, *.hpp)
vgrep --dart Widget      # Dart files (*.dart)
vgrep --swift View       # Swift files (*.swift)
```

### 3. Function Search Shorthand
Append `_fn` to quickly search for function definitions:
```bash
vgrep handleRequest_fn
# In Go projects: converts to `func handleRequest`
# In Rust projects: converts to `fn handleRequest`
```

### 4. Interactive Project History
Run `vgrep` without arguments inside any repository to view and select from recent search queries:
```bash
vgrep
```
*Presents an interactive `fzf` or numbered list ranked by frequency and relative recency (e.g., `just now`, `10m ago`, `2d ago`).*

### 5. Review Previous Search Results
Re-open the last search results saved in the session cache without running `rg` again:
```bash
vgrep -v
# or
vgrep --view
```

### 6. Health Check
Check your installed dependencies, editors, and config paths:
```bash
vgrep --health
```

### 7. Edit Configuration
Open `~/.config/vgrep/config.toml` in your `$EDITOR`:
```bash
vgrep -e
# or
vgrep --edit
```

### 8. Clear History & Cache
Wipe project history and cached session files:
```bash
vgrep --init
```

---

## TUI Keyboard Shortcuts & Vim Motions

When multiple matches are found, `vgrep` enters the interactive alternate-screen TUI:

```text
 vgrep  filter: 
>  0 ~/project/main.go
   1     12: func initialize() {
   2     48: func handleRequest() {
   3 ~/project/router.go
   4      8: func registerRoutes() {

[j/k, <num>j/k, J/K (files), g/G, / (filter), r (rgr replace), Enter/o (open), q (quit)]
```

| Key | Action |
|---|---|
| `j` / `k` | Move cursor down / up by 1 row |
| `<num>j` / `<num>k` | Jump down / up by `<num>` rows (e.g., `3j`, `5k`) |
| `J` (`Shift+j`) | Jump to the **first match of the next file** |
| `K` (`Shift+k`) | Jump to the **first match of the previous file** |
| `g` | Jump to top (file header at row 0) |
| `G` | Jump to bottom |
| `/` | Enter live filter mode (shows red block cursor; press `Enter`/`Esc` to exit filter) |
| `r` | Launch `rgr` (repgrep) for interactive search and replace (hidden if `rgr` is not installed) |
| `<Enter>` / `o` | Open file at line in `$EDITOR` (returns back to TUI after `:q`) |
| `q` / `Ctrl+c` | Exit `vgrep` and restore original terminal screen cleanly |

---

## Editor Configuration

### Setting Default Editor
`vgrep` respects the `$EDITOR` environment variable. If unset, it automatically tries `wig` $\rightarrow$ `nvim` $\rightarrow$ `vim`.

Set your preferred editor in your shell profile (`~/.bashrc` or `~/.zshrc`):
```bash
export EDITOR="wig"
# or
export EDITOR="nvim"
```

### `wig` Shared Session Integration
Every search written by `vgrep` is saved to:
```text
~/.config/wig/rg_search.json
```
This enables `wig` to load the exact search result list into its quickfix/search list buffer for in-editor navigation (`:cnext` / `:cprev`).

---

## Session & `rg_search.json` Format

Unlike standard `rg --json` (which emits a stream of newline-delimited JSON events like `begin`, `match`, `end`, and `summary`), `vgrep` aggregates, filters, and transforms results into a **clean root JSON array** stored at `~/.config/wig/rg_search.json`.

```text
[ vgrep query ] ──► [ rg --json -g <globs> <pattern> ]
                             │
                      (NDJSON Stream)
                             │
                             ▼
              [ Decode RgMessage / RgMatchData ]
               - FilePath (normalized to absolute path)
               - LineNumber
               - Submatches[0].Start (character column)
               - Lines.Text (matching line content)
                             │
                             ▼
                 [ Serialize to JSON Array ]
                             │
                             ▼
             ~/.config/wig/rg_search.json
```

### JSON Schema

```json
[
  {
    "file_path": "/opt/ai/gh/vig/cmd/main.go",
    "line": 94,
    "char": 76,
    "text": "\tkeys := wig.NewKeyHandler(config.DefaultKeyMap(editorCfg.Leader, editorCfg.CommentStyle))\n"
  },
  {
    "file_path": "/opt/ai/gh/vig/editor.go",
    "line": 14,
    "char": 1,
    "text": "\tCommentStyle        string `toml:\"comment_style\"`\n"
  }
]
```

### Field Definitions

| Field | Type | Description |
|---|---|---|
| `file_path` | `string` | Fully-qualified absolute path to the file |
| `line` | `int` | 1-based line number of the match |
| `char` | `int` | 0-based character column offset of the match start |
| `text` | `string` | Raw line content (including original whitespace/newlines) |

### Session Re-use (`-v` / `--view`)
When `vgrep -v` is invoked, `vgrep` skips running `ripgrep` entirely and loads this JSON session file directly into the interactive TUI for instant replay and navigation.

---

## Configuration File (`~/.config/vgrep/config.toml`)

```toml
# Default editor command
editor = "wig"

# Custom path to shared search session JSON
session_file = "~/.config/wig/rg_search.json"
```

---

## License

MIT License. Feel free to use and customize for your workflows!

