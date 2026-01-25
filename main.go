package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type SystemStats struct {
	CPUUsage    float64
	GPUUsage    float64
	MemoryUsage float64
	GPUMemory   float64
	CPUCores    []float64
	Processes   []ProcessInfo
}

type ProcessInfo struct {
	PID     int32
	CPU     float64
	Memory  float32
	Command string
}

func main() {
	app := tview.NewApplication()
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)

	textView.SetChangedFunc(func() {
		app.Draw()
	})

	// Handle quit keys (q, Q, or Escape)
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Rune() == 'Q' {
			app.Stop()
			return nil
		}
		return event
	})

	// Update stats every 3 seconds
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			stats := collectStats()
			display := formatOutput(stats)
			textView.SetText(display)
			<-ticker.C
		}
	}()

	if err := app.SetRoot(textView, true).SetFocus(textView).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}

func collectStats() SystemStats {
	stats := SystemStats{}

	// Get per-core CPU usage (more efficient - one call gets all cores)
	perCoreCPU, _ := cpu.Percent(time.Second, true)
	stats.CPUCores = perCoreCPU

	// Calculate average CPU usage from per-core data
	if len(perCoreCPU) > 0 {
		var sum float64
		for _, val := range perCoreCPU {
			sum += val
		}
		stats.CPUUsage = sum / float64(len(perCoreCPU))
	}

	// Memory Usage
	memInfo, _ := mem.VirtualMemory()
	if memInfo != nil {
		stats.MemoryUsage = memInfo.UsedPercent
	}

	// GPU stats
	stats.GPUUsage = getGPUUsage()
	stats.GPUMemory = getGPUMemory()

	// Process list
	stats.Processes = getTopProcesses()

	return stats
}

func getGPUUsage() float64 {
	// Try to get GPU usage from nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	usageStr := strings.TrimSpace(string(output))
	usage, err := strconv.ParseFloat(usageStr, 64)
	if err != nil {
		return 0.0
	}

	return usage
}

func getGPUMemory() float64 {
	// Try to get GPU memory usage from nvidia-smi
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	// Parse "used, total" format
	parts := strings.Split(strings.TrimSpace(string(output)), ", ")
	if len(parts) != 2 {
		return 0.0
	}

	used, err1 := strconv.ParseFloat(parts[0], 64)
	total, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || total == 0 {
		return 0.0
	}

	return (used / total) * 100.0
}

func getTopProcesses() []ProcessInfo {
	processes, _ := process.Processes()
	var procInfos []ProcessInfo

	// Get CPU percentages with a small interval for accuracy
	for _, p := range processes {
		// CPUPercent needs an interval - use 0 for immediate snapshot
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			continue
		}

		memPercent, err := p.MemoryPercent()
		if err != nil {
			continue
		}

		// Skip processes with 0 CPU usage to focus on active ones
		if cpuPercent == 0 {
			continue
		}

		exe, err := p.Exe()
		if err != nil {
			// Fallback to process name
			name, _ := p.Name()
			exe = name
		}

		procInfos = append(procInfos, ProcessInfo{
			PID:     p.Pid,
			CPU:     cpuPercent,
			Memory:  memPercent,
			Command: exe,
		})
	}

	// Sort by CPU usage descending
	sort.Slice(procInfos, func(i, j int) bool {
		return procInfos[i].CPU > procInfos[j].CPU
	})

	// Return top 10
	if len(procInfos) > 10 {
		procInfos = procInfos[:10]
	}

	return procInfos
}

// getColorForPercent returns a color tag based on percentage thresholds
func getColorForPercent(percent float64) string {
	if percent < 50.0 {
		return "[green]"
	} else if percent < 80.0 {
		return "[yellow]"
	} else {
		return "[red]"
	}
}

func formatOutput(stats SystemStats) string {
	var output string

	// Header line - match exact format from example with color coding
	cpuColor := getColorForPercent(stats.CPUUsage)
	gpuColor := getColorForPercent(stats.GPUUsage)
	memColor := getColorForPercent(stats.MemoryUsage)
	gpuMemColor := getColorForPercent(stats.GPUMemory)

	output += fmt.Sprintf("CPU Usage:    %s%5.1f%%[-]    GPU Usage:     %s%3.0f%%[-]\n",
		cpuColor, stats.CPUUsage, gpuColor, stats.GPUUsage)
	output += fmt.Sprintf("Memory:       %s%5.1f%%[-]    GPU Memory:   %s%4.1f%%[-]\n",
		memColor, stats.MemoryUsage, gpuMemColor, stats.GPUMemory)
	output += "\n"

	// CPU cores - format exactly as in example (4 cores per line) with color coding
	coreCount := len(stats.CPUCores)
	for i := 0; i < coreCount; i += 4 {
		line := ""
		for j := 0; j < 4 && i+j < coreCount; j++ {
			coreNum := i + j
			corePercent := stats.CPUCores[coreNum]
			coreColor := getColorForPercent(corePercent)
			if j < 2 {
				line += fmt.Sprintf("CPU%02d: %s%4.1f%%[-]  ", coreNum, coreColor, corePercent)
			} else if j == 2 {
				line += fmt.Sprintf("CPU%02d: %s%4.1f%%[-]   ", coreNum, coreColor, corePercent)
			} else {
				line += fmt.Sprintf("CPU%02d: %s%4.1f%%[-]", coreNum, coreColor, corePercent)
			}
		}
		output += line + "\n"
	}
	output += "\n"

	// Process list header
	output += fmt.Sprintf("%-10s %5s  %5s  %s\n", "PID", "CPU%", "MEM%", "COMMAND")

	// Process list with color coding
	for _, proc := range stats.Processes {
		cpuColor := getColorForPercent(proc.CPU)
		memColor := getColorForPercent(float64(proc.Memory))
		output += fmt.Sprintf("%-10d %s%5.1f[-]  %s%5.1f[-]  %s\n",
			proc.PID, cpuColor, proc.CPU, memColor, proc.Memory, proc.Command)
	}

	return output
}
