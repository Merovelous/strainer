# strainer

[![CI](https://github.com/Merovelous/strainer/actions/workflows/ci.yml/badge.svg)](https://github.com/Merovelous/strainer/actions/workflows/ci.yml)

Fast wordlist filter with an interactive terminal UI. Browse files and archives, configure filters, watch it process in real time.

Built for the preparation work before handing a wordlist to hashcat or john — filtering by length, charset, regex, and deduplication.

---

## Install

**From source (Go 1.24+):**

```bash
git clone https://github.com/Merovelous/strainer
cd strainer
go build -o strainer ./cmd/strainer/
```

**Or directly:**

```bash
go install github.com/Merovelous/strainer/cmd/strainer@latest
```

> **Dependency:** [`7-Zip`](https://www.7-zip.org/) (`7z` in PATH) is required only for archive extraction.

---

## TUI

```bash
./strainer
```

Four steps — the title bar always shows where you are:

```
⚡ strainer v1.7.0       ✓ Select  ›  ● Filters  ›  ○ Process  ›  ○ Done
```

1. **Select** — browse the filesystem. Archives (`.7z`, `.zip`, `.tar.gz`, ...) are highlighted. Select one to pick a file from inside it.
2. **Filters** — configure what to keep. Options are grouped by category; a description appears next to each one when focused.
3. **Process** — live progress bar, throughput, ETA, lines kept/dropped.
4. **Done** — output path, retention rate, and stats. Press `r` to process another file.

### Controls

| Key | Action |
|---|---|
| `↑` `↓` | Navigate |
| `Space` | Toggle filter on/off |
| `←` `→` | Cycle through options (bloom size) |
| `Enter` | Edit value (length, regex) |
| `⌫` | Parent directory (browser) |
| `Esc` | Back |
| `q` | Back / Quit |
| `r` | Process another file (summary screen) |

---

## CLI

For scripting and batch use. All filters available as flags.

```bash
strainer --input <file> --output <file> [filters]

Required:
  --input  <file>    Wordlist file or archive (.7z .zip .tar.gz ...)
  --output <file>    Output file path

Archive:
  --entry  <name>    File to extract from the archive

Filters:
  --min    <n>       Keep lines with length >= n
  --max    <n>       Keep lines with length <= n
  --ascii            Keep ASCII-printable lines only (0x20–0x7E)
  --regex  <pat>     Keep lines matching regex pattern
  --dedup            Deduplicate lines
  --bloom-size <s>   Bloom filter RAM for dedup on large files (e.g. 1g, 4g)

Other:
  --quiet            Suppress progress output
```

### Examples

```bash
# Keep passwords 8–12 chars, printable ASCII only
strainer --input rockyou.txt --output out.txt --min 8 --max 12 --ascii

# Extract and filter from an archive
strainer --input dump.7z --entry passwords.txt --output out.txt --min 8 --ascii

# Deduplicate a large file with bloom filter (trades a small false-positive rate for low RAM)
strainer --input huge.txt --output deduped.txt --dedup --bloom-size 4g

# Batch loop
for f in /wordlists/*.7z; do
  strainer --input "$f" --entry rockyou.txt \
           --min 8 --max 16 --ascii \
           --output "${f%.7z}_filtered.txt" --quiet
done
```

---

## Filters

All filters are applied together — a line must pass every enabled check to be written.

| Filter | Flag | What it does |
|---|---|---|
| Min length | `--min N` | Drop lines shorter than N bytes |
| Max length | `--max N` | Drop lines longer than N bytes |
| ASCII only | `--ascii` | Keep lines where every byte is in `[\x20–\x7E]` (printable, no control chars) |
| Regex | `--regex PAT` | Keep lines matching a Go regexp |
| Deduplicate | `--dedup` | Drop duplicate lines — first occurrence wins |

### Deduplication strategy

| File type | Method | Accuracy |
|---|---|---|
| Plain file ≤ 4 GB | mmap + sorted offsets | 100% |
| Plain file > 4 GB | In-memory map or bloom filter | 100% / probabilistic |
| Archive | In-memory map or bloom filter | 100% / probabilistic |

Use `--bloom-size` with `--dedup` on large files to cap RAM usage. Presets: `256m`, `512m`, `1g`, `4g`, `8g`, or any custom value like `16g`, `2048m`.

---

## Performance

Filtering a 10 GB synthetic wordlist (859 M lines) — keep 8–12 char printable ASCII lines (~40% pass rate):

| Tool | Time | Throughput |
|---|---|---|
| **strainer** | **28.8 s** | **~347 MB/s** |
| ripgrep 15.1 | 51.4 s | ~194 MB/s |

~1.8× faster than ripgrep on equivalent work. The pipeline runs on a single goroutine with no per-line allocations: `bufio.Scanner` → `filterLine()` (byte-level checks) → `bufio.Writer`. No regex engine overhead for length and charset checks.

*Environment: AMD Ryzen 7 5800H, Linux 6.18, Go 1.24, ripgrep 15.1.0*

---

## Why not grep / ripgrep?

`rg` and `grep` are the right tools if you have a file path and a regex ready. strainer is for the interactive, archive-heavy workflow where you don't:

- **Archives:** filtering a file inside a `.7z` with `rg` requires a manual `7z x -so ... 2>/dev/null | rg ... | pv > out.txt` pipeline. strainer makes it four keystrokes.
- **Feedback:** `rg` gives no progress on a 10 GB file. strainer shows bytes read, throughput, ETA, and retention in real time.
- **Output naming:** strainer generates a name from the active filters (`rockyou_8to12_ascii.txt`). No manual renaming.
- **Browse:** when an archive has dozens of files you need to `7z l` it first. strainer lets you browse and pick interactively.

---

## License

MIT — see [LICENSE](LICENSE).
