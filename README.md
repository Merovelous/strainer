# strainer

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
- **Filter configuration** — min length, max length, ASCII-only (`[\x20-\x7E]`)
- **Live metrics** — throughput, lines read/kept/dropped, CPU%, RAM (RSS), I/O bytes
- **Progress bar** — determinate for plain files, indeterminate for archives
- **Smart output naming** — filename derived from source and active filters
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

```bash
./strainer
```

Launch the TUI, browse to a wordlist file or archive, configure filters, and press Tab to start processing.

### Controls

| Key | Action |
|---|---|
| `↑` `↓` or `j` `k` | Navigate |
| `Enter` / `Space` | Select file or toggle filter |
| `e` | Edit filter value |
| `Backspace` | Go to parent directory |
| `Tab` | Confirm filters and start processing |
| `Esc` | Back (from filter screen) |
| `q` | Back / Quit |
| `r` | Process another file (from summary) |

### Example

1. Launch `./strainer`
2. Browse to `weakpass_4a.txt.7z`
3. Select `rockyou.txt` from the archive contents
4. Set **Min Length** = 8, **Max Length** = 12, enable **ASCII Only**
5. Press `Tab` — output: `rockyou_8to12_ascii.txt`

---

## Pipeline

Processing uses a single-goroutine, zero-allocation pipeline:

```
source (file or 7z stdout)
  → bufio.Scanner (64 KB buffer, 1 MB max line)
  → filterLine()  (byte-level checks, no string allocation)
  → bufio.Writer  (256 KB write buffer)
  → output file
```

Metrics (CPU%, RSS, I/O) are sampled from `/proc/self` every 100ms on a separate goroutine and displayed without blocking the pipeline. All counters use `sync/atomic`.

---

## Filters

| Filter | Description |
|---|---|
| Min Length | Drop lines shorter than N bytes |
| Max Length | Drop lines longer than N bytes |
| ASCII Only | Keep only lines where every byte is in `[\x20-\x7E]` (printable ASCII, no control chars) |

Filters combine: a line must pass all enabled checks to be written to output.

---

## License

MIT — see [LICENSE](LICENSE).
