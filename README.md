# ⚡ wordlist-forge

Go TUI wordlist processor. Replaces the `7z | pv | rg | pv > file` pipeline with a single interactive binary.

## Features

- 📁 **File Browser** — navigate dirs, archives highlighted with 📦
- 📦 **Archive Support** — auto-detects `.7z`, `.zip`, `.tar.gz`, `.rar` etc.
- ⚙️ **Filter Config** — min/max length + ASCII-only (`[\x20-\x7E]`)
- 🔄 **Concurrent Pipeline** — `bufio.Scanner` + worker pool (`NumCPU` goroutines)
- 📊 **Live Metrics** — CPU%, RAM (RSS), IO read/write from `/proc/self`
- 🏷️ **Smart Naming** — output named from applied rules (`pass_8to12_ascii.txt`)

## Install

```bash
go build -o wordlist-forge .
```

## Usage

```bash
./wordlist-forge
```

### Controls

| Key | Action |
|-----|--------|
| `↑↓` / `jk` | Navigate |
| `Enter` / `Space` | Select / Toggle |
| `e` | Edit filter value |
| `Tab` | Start processing |
| `q` | Quit / Back |
| `Esc` | Back (from filters) |
| `r` | Restart (from summary) |

### Example

Select `weakpass_4a.txt.7z` → choose file inside → set min=8, max=12, ascii-only → output: `weakpass_4a_8to12_ascii.txt`

## Pipeline

```
7z/stdin → [reader goroutine] → lineChan → [N filter goroutines] → resultChan → [writer goroutine] → file
```

- **Reader**: `bufio.Scanner` with 64KB buffer, 1MB max line
- **Filter workers**: `runtime.NumCPU` goroutines, length + ASCII checks
- **Writer**: batched writes (256KB buffer, flush every 64KB)
- **Metrics**: atomic counters, polled every 100ms from `/proc/self`
