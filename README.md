# sysmon

A terminal-based system monitor written in Go that displays CPU, memory, GPU, and process information in real-time.

## Features

- Real-time CPU usage (overall and per-core)
- Memory usage statistics
- GPU usage and memory (via nvidia-smi, if available)
- Top processes by CPU usage
- Clean, readable terminal interface

## Requirements

- Go 1.21 or later
- Linux system (for system stats)
- nvidia-smi (optional, for GPU stats)

## Installation

```bash
go mod download
go build -o sysmon
```

## Usage

```bash
./sysmon
```

Press `q` or `Ctrl+C` to exit.

## Output Format

The TUI displays:
- CPU and GPU usage percentages
- Memory and GPU memory percentages
- Per-core CPU usage (4 cores per line)
- Top 10 processes by CPU usage (PID, CPU%, MEM%, COMMAND)

See `EXAMPLE_OUTPUT.md` for an example of the output format.