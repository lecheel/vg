### 1. Triggering the History Picker
Simply run `vgrep` **without any arguments** inside any workspace or git repository:

```bash
vgrep
```

---

### 2. Automatic Project-Root Scoping
Unlike shell history (which mixes everything into one global file like `~/.bash_history`), `vgrep` automatically detects your project root by walking upward looking for project marker files:
- `.git`
- `go.mod`
- `Cargo.toml`
- `package.json`
- `pyproject.toml`
- `pubspec.yaml`

**Benefit**: Searches from Project A never clutter the history of Project B. Each repository has its own isolated history.

---

### 3. Interactive Selection Interface

`vgrep` presents your previous searches ranked by **frequency (hit count)** and **relative recency** (`just now`, `10m ago`, `2d ago`).

#### Mode A: With `fzf` installed (Recommended)
If `fzf` is in your `$PATH`, an interactive fuzzy picker opens:

```text
Project: my-project
> 
  00 | vgrep view (like vgrep -v)
  01 | [8 hits] (just now) handleRequest
  02 | [5 hits] (15m ago) {"error": "unauthorized"}
  03 | [3 hits] (2d ago) type Config struct
```

- **Fuzzy Search**: Type any snippet to filter past queries.
- **Quotes & Spaces Preserved**: Complex queries with quotes (`'...'`, `"..."`) or spaces appear verbatim without broken escapes.
- **Option `00`**: Instantly reloads the previous search results (`vgrep -v`) directly from the session cache without re-running ripgrep.
- **Select with Enter**: Selecting any pattern immediately executes the search and enters the TUI (or jumps straight to your editor if only 1 match is found).

#### Mode B: Without `fzf` (Built-in Fallback)
If `fzf` is not installed, `vgrep` displays a clean numbered terminal menu:

```text
Saved searches for [my-project]:
  [0] vgrep view (like vgrep -v)
  [1] (just now) handleRequest
  [2] (15m ago) {"error": "unauthorized"}
  [3] (2d ago) type Config struct

Select index (Enter to cancel): 
```
Type the index number (e.g. `1` or `0`) and hit `Enter`.

---

### 4. How Searches Are Recorded

Searches are recorded automatically into `~/.config/vgrep/history.json` in two places:

1. **From the CLI**:
   ```bash
   vgrep 'my_complex_search_query'
   ```
   Recorded as soon as the command runs.

2. **From Inside the TUI (`n`)**:
   - While viewing search results in the TUI, press `n` to open the raw search prompt.
   - Type a new pattern and press `Enter`.
   - The new pattern is automatically saved to the active project's history.

#### Storage Rules:
- **Frequency Tracking**: Searching an existing query increments its hit count (`UseCount`) and updates its timestamp to the current time.
- **Automatic Capping**: History is capped at the **25 most relevant patterns** per repository to keep the list fast and concise.

---

### 5. Summary Workflow Diagram

```text
               $ vgrep (no arguments)
                         │
              [ Find Project Root ]
      (.git, go.mod, Cargo.toml, package.json...)
                         │
        [ Read ~/.config/vgrep/history.json ]
       (Filtered to current repository root)
                         │
              Is `fzf` installed?
               ├── Yes ──► Interactive `fzf` picker
               └── No  ──► Numbered CLI prompt
                         │
                 Selection made:
               ├── Option [0]  ──► Re-open last session (`vgrep -v`)
               └── Pattern [N] ──► Run `rg` on chosen pattern
                                         │
                             ┌───────────┴───────────┐
                       Only 1 Match?           Multiple Matches?
                             │                       │
                     Jump straight to           Open Vim-Motion
                    $EDITOR +<line>:<col>             TUI
```

---

### 6. Management Commands

| Action | Command |
|---|---|
| **View history file path** | `vgrep --health` (shows history location, default: `~/.config/vgrep/history.json`) |
| **Inspect raw history data** | `cat ~/.config/vgrep/history.json` |
| **Clear all history & cache** | `vgrep --init` |
