# strainer

[![CI](https://github.com/Merovelous/strainer/actions/workflows/ci.yml/badge.svg)](https://github.com/Merovelous/strainer/actions/workflows/ci.yml)

A terminal UI for filtering and processing large wordlists. Handles plain files and compressed archives, applies configurable rules, and streams output with live throughput metrics.

## Why not just use ripgrep or grep?

`rg`, `grep`, and `awk` are excellent line-processing tools, but composing them for wordlist work has real friction:

**Archive handling requires manual steps.** To filter a file inside a `.7z` archive you need to know its internal path, construct a shell pipeline, and suppress 7-zip's stderr manually:

```bash
7z x -so wordlists.7z rockyou.txt 2>/dev/null \
  | rg '^[\x20-\x7E]{8,12}$' \
  | pv > rockyou_8to12_ascii.txt
```

strainer makes this a four-keystroke workflow: browse to the archive, pick the file, set the filters, press Tab.

**No feedback on multi-GB files.** `rg` gives no progress on a 10 GB wordlist. You wait, then you get output. strainer shows bytes read, lines kept/dropped, throughput, CPU, and RAM in real time.

**Output naming is manual.** After filtering you rename the output to something meaningful. strainer generates the name automatically from the rules you set (`rockyou_8to12_ascii.txt`, `weakpass_min8.txt`).

**No browse.** When an archive contains dozens of wordlists you need to `7z l` it, find the filename, then reference it exactly. strainer lets you browse archive contents directly.

That said — if you already have the file path, the filter as a regex, and you don't need metrics, `rg` is the right tool. strainer targets the interactive, archive-heavy preparation workflow that comes before handing a wordlist to hashcat or john.

---

## Features

- **File browser** — navigate the filesystem, archives auto-detected and highlighted
- **Archive support** — `.7z`, `.zip`, `.tar.gz`, `.tar.bz2`, `.tar.xz`, `.rar`, `.tgz`, `.xz`
- **Filter configuration** — min/max length, ASCII-only, regex pattern, deduplication
- **Live metrics** — throughput, lines read/kept/dropped, CPU%, RAM (RSS), I/O bytes
- **Progress bar** — determinate for plain files, indeterminate for archives
- **Smart output naming** — filename derived from source and active filters
- **Multi-core filtering** — parallel workers with `--jobs N` (default: all cores); output order preserved
- **CLI mode** — `--input`, `--output`, `--min`, `--max`, `--ascii`, `--regex`, `--dedup`, `--jobs` flags for scripting
- **Single binary** — no runtime dependencies beyond `7z` for archive extraction

---

## Requirements

- Go 1.21+
- [`7-Zip`](https://www.7-zip.org/) (`7z` in PATH) — only required for archive extraction

---

## Install

```bash
git clone https://github.com/Merovelous/strainer
cd strainer
go build -o strainer .
```

Or install directly:

```bash
go install github.com/Merovelous/strainer@latest
```

---

## Usage

### TUI

```bash
./strainer
```

Launch the TUI, browse to a wordlist file or archive, configure filters, navigate to **▶ Start Processing** and press `Enter`.

### Controls

| Key | Action |
|---|---|
| `↑` `↓` or `j` `k` | Navigate |
| `Enter` / `Space` | Select file or toggle filter |
| `e` | Edit filter value |
| `Backspace` | Go to parent directory |
| `Tab` | Quick-start (jump straight to processing) |
| `Esc` | Back (from filter screen) |
| `q` | Back / Quit |
| `r` | Process another file (from summary) |

### TUI example

1. Launch `./strainer`
2. Browse to `weakpass_4a.txt.7z`
3. Select `rockyou.txt` from the archive contents
4. Set **Min Length** = 8, **Max Length** = 12, enable **ASCII Only**
5. Navigate to **▶ Start Processing** and press `Enter` — output: `rockyou_8to12_ascii.txt`

### CLI mode

```bash
# Plain file
strainer --input rockyou.txt --min 8 --max 12 --ascii --output rockyou_8to12_ascii.txt

# File inside an archive
strainer --input weakpass.7z --file rockyou.txt --min 8 --ascii --output out.txt

# With regex and deduplication
strainer --input dump.txt --regex '^[a-zA-Z0-9]{8,}$' --dedup --output clean.txt

# Quiet (no progress, summary line only)
strainer --input dump.txt --min 8 --output out.txt --quiet

# Batch loop
for f in /mnt/wordlists/*.7z; do
  strainer --input "$f" --file rockyou.txt --min 8 --max 16 --ascii \
           --output "${f%.7z}_filtered.txt" --quiet
done
```

---

## Pipeline

**Single-core** (default when `--dedup` is active or `--jobs 1`):

```
source (file or 7z stdout)
  → bufio.Scanner (64 KB buffer, 1 MB max line)
  → filterLine()  (byte-level checks, no string allocation)
  → bufio.Writer  (256 KB write buffer)
  → output file
```

**Multi-core** (default for non-dedup workloads, `--jobs N` or 0 for auto):

```
source
  → readChunks()  (4 MB blocks, newline-aligned, sequence-numbered)
  → N worker goroutines  (each runs filterLine on its chunk)
  → writeOrdered()  (min-heap reorders by sequence, writes in order)
  → output file
```

Metrics (CPU%, RSS, I/O) are sampled from `/proc/self` every 100ms on a separate goroutine and displayed without blocking the pipeline. All counters use `sync/atomic`.

---

## Filters

| Filter | Flag | Description |
|---|---|---|
| Min Length | `--min N` | Drop lines shorter than N bytes |
| Max Length | `--max N` | Drop lines longer than N bytes |
| ASCII Only | `--ascii` | Keep only lines where every byte is in `[\x20-\x7E]` (printable ASCII, no control chars) |
| Regex Match | `--regex PATTERN` | Keep only lines matching the Go regexp pattern |
| Deduplicate | `--dedup` | Drop duplicate lines (first occurrence wins; uses in-memory map) |

Filters combine: a line must pass all enabled checks to be written to output.

---

## Performance

Task: filter a 9.8 GB synthetic wordlist (859 M lines) — keep lines where length is 8–12 **and** all bytes are printable ASCII `[\x20-\x7E]`. 40% of lines pass the filter.

| Tool | Command | Time | Throughput |
|---|---|---|---|
| **strainer (16 cores)** | `strainer --input bench.txt --min 8 --max 12 --ascii --output /dev/null --quiet` | **26.3 s** | **~372 MB/s** |
| **strainer (1 core)** | `strainer ... --jobs 1` | **47.7 s** | **~205 MB/s** |
| ripgrep 15.1 | `rg '^[\x20-\x7E]{8,12}$' bench.txt > /dev/null` | 51.4 s | ~194 MB/s |

**strainer (multi-core) is ~2× faster than ripgrep** on this workload.

Why strainer wins here:
- `filterLine` works on `[]byte` directly — no string allocation per line
- Length check = two integer comparisons; ASCII check = one byte-range loop — no regex engine overhead
- 4 MB read chunks — optimal for sequential I/O, fewer syscalls
- N worker goroutines filter chunks in parallel; output order is preserved via sequence numbers + min-heap

ripgrep pays per-line cost for regex compilation context, UTF-8 validation, and match extraction even when the filter is trivially simple.

**Environment:** AMD Ryzen 7 5800H, 6.6 GB RAM, virtual disk (HDD-backed), Linux 6.18, Go 1.26.3, ripgrep 15.1.0

To reproduce:

```bash
cd bench
go run gen_bench.go -size 10000 -out bench.txt
bash run_bench.sh
```

`run_bench.sh` uses `hyperfine` for multi-run statistics if available, otherwise falls back to `time`.

---

## License

MIT — see [LICENSE](LICENSE).
