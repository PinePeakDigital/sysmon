# sysmon

A terminal-based system monitor written in Go that displays CPU, memory, GPU, and process information in real-time.

## Features

- Real-time CPU usage (overall and per-core)
- Memory usage statistics
- GPU usage and memory (via nvidia-smi for NVIDIA or rocm-smi for AMD, if available)
- Top processes by CPU usage
- Clean, readable terminal interface

## Requirements

- Go 1.21 or later
- Linux system (for system stats)
- nvidia-smi (optional, for NVIDIA GPU stats)
- rocm-smi (optional, for AMD GPU stats)

## Installation

### Using bin (Recommended)

[bin](https://github.com/marcosnils/bin) is a lightweight binary manager that makes installing and updating sysmon easy:

```bash
# Install bin if you don't have it
curl -sSfL https://raw.githubusercontent.com/marcosnils/bin/master/install.sh | sh

# Install sysmon using bin
bin install github.com/PinePeakDigital/sysmon
```

After installation, `sysmon` will be available in your `PATH` (typically `~/.local/bin/` on Linux/macOS).

### From Source

```bash
go mod tidy
go build -o sysmon
```

> **macOS note:** per-core CPU stats require cgo (gopsutil reads them via
> `host_processor_info`). A native `go build` enables cgo by default, so this
> works out of the box. If you build with `CGO_ENABLED=0`, the per-core view
> will be empty and only aggregate CPU usage is shown.

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
