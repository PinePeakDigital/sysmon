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
			// Get terminal width - try multiple methods
			width := 80 // Default fallback
			
			// Try 1: GetInnerRect from TextView
			_, _, w, _ := textView.GetInnerRect()
			if w > 0 {
				width = w
			} else {
				// Try 2: Environment variable COLUMNS
				if colsStr := os.Getenv("COLUMNS"); colsStr != "" {
					if cols, err := strconv.Atoi(colsStr); err == nil && cols > 0 {
						width = cols
					}
				}
				
				// Try 3: Use tcell to get screen size
				if width == 80 {
					if screen, err := tcell.NewScreen(); err == nil {
						if err := screen.Init(); err == nil {
							screenWidth, _ := screen.Size()
							screen.Fini()
							if screenWidth > 0 {
								width = screenWidth
							}
						}
					}
				}
			}
			
			display := formatOutput(stats, width)
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

// getPercentBar returns a visual bar representation of a percentage
func getPercentBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	
	// Clamp percent to 0-100
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}
	
	// Calculate filled blocks
	filled := int((percent / 100.0) * float64(width))
	bar := ""
	
	// Use Unicode block characters for a smooth bar
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	for i := filled; i < width; i++ {
		bar += "░"
	}
	
	return bar
}

// getPercentBarWithText overlays text on top of a percentage bar
// Uses ANSI escape sequences and TranslateANSI to create background color effect
// Adds boundary characters to delineate the bar edges
// Label is left-aligned, percentage is right-aligned
func getPercentBarWithText(label, percentText, colorCode string, percent float64, barWidth int) string {
	if barWidth <= 0 {
		return label + percentText
	}
	
	// Clamp percent to 0-100
	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}
	
	// Calculate filled blocks
	filled := int((percent / 100.0) * float64(barWidth))
	labelRunes := []rune(label)
	percentRunes := []rune(percentText)
	labelLen := len(labelRunes)
	percentLen := len(percentRunes)
	totalTextLen := labelLen + percentLen
	
	// If text is longer than bar width, just return text with color
	if totalTextLen >= barWidth {
		return colorCode + label + percentText + "[-]"
	}
	
	// Build the result with ANSI background colors
	result := ""
	
	// Determine ANSI background color code for filled portion
	var bgANSI string
	if colorCode == "[green]" {
		bgANSI = "\033[42m" // Green background
	} else if colorCode == "[yellow]" {
		bgANSI = "\033[43m" // Yellow background
	} else if colorCode == "[red]" {
		bgANSI = "\033[41m" // Red background
	}
	resetANSI := "\033[0m"
	
	// Add left boundary character with color
	result += colorCode + "[" + "[-]"
	
	// Calculate where percentage starts (right-aligned)
	percentStart := barWidth - percentLen
	
	// Build bar with label left-aligned and percentage right-aligned
	for i := 0; i < barWidth; i++ {
		if i < labelLen {
			// Label portion (left-aligned)
			if i < filled {
				// Label is on filled portion - overlay with background color
				result += bgANSI + colorCode + string(labelRunes[i]) + "[-]" + resetANSI
			} else {
				// Label is on unfilled portion - show without background
				result += colorCode + string(labelRunes[i]) + "[-]"
			}
		} else if i < percentStart {
			// Middle portion (between label and percentage) - bar only
			if i < filled {
				// Show filled bar with background
				result += bgANSI + " " + resetANSI
			} else {
				// Show space (transparent) to use terminal's background
				result += " "
			}
		} else {
			// Percentage portion (right-aligned)
			percentIdx := i - percentStart
			if i < filled {
				// Percentage is on filled portion - overlay with background color
				result += bgANSI + colorCode + string(percentRunes[percentIdx]) + "[-]" + resetANSI
			} else {
				// Percentage is on unfilled portion - show without background
				result += colorCode + string(percentRunes[percentIdx]) + "[-]"
			}
		}
	}
	
	// Add right boundary character with color
	result += colorCode + "]" + "[-]"
	
	// Translate ANSI codes to tview format
	return tview.TranslateANSI(result)
}

func formatOutput(stats SystemStats, terminalWidth int) string {
	var output string

	// Header line - labels inside bars like CPU cores
	cpuColor := getColorForPercent(stats.CPUUsage)
	gpuColor := getColorForPercent(stats.GPUUsage)
	memColor := getColorForPercent(stats.MemoryUsage)
	gpuMemColor := getColorForPercent(stats.GPUMemory)

	cpuLabel := "CPU Usage"
	cpuPercent := fmt.Sprintf("%5.1f%%", stats.CPUUsage)
	gpuLabel := "GPU Usage"
	gpuPercent := fmt.Sprintf("%3.0f%%", stats.GPUUsage)
	memLabel := "Memory"
	memPercent := fmt.Sprintf("%5.1f%%", stats.MemoryUsage)
	gpuMemLabel := "GPU Memory"
	gpuMemPercent := fmt.Sprintf("%4.1f%%", stats.GPUMemory)

	cpuBarWithText := getPercentBarWithText(cpuLabel, cpuPercent, cpuColor, stats.CPUUsage, 25)
	gpuBarWithText := getPercentBarWithText(gpuLabel, gpuPercent, gpuColor, stats.GPUUsage, 25)
	memBarWithText := getPercentBarWithText(memLabel, memPercent, memColor, stats.MemoryUsage, 25)
	gpuMemBarWithText := getPercentBarWithText(gpuMemLabel, gpuMemPercent, gpuMemColor, stats.GPUMemory, 25)

	output += fmt.Sprintf("%s %s\n", cpuBarWithText, gpuBarWithText)
	output += fmt.Sprintf("%s %s\n", memBarWithText, gpuMemBarWithText)
	output += "\n"

	// CPU cores - calculate width based on terminal width
	coreCount := len(stats.CPUCores)
	
	// Calculate cores per line and bar width
	// Each bar format: [ + label (CPU00) + barWidth (total content width) + percentage (100.0%) + ]
	// Label: "CPU00" = 5 chars, Percentage: "100.0%" = 6 chars, Brackets: 2 chars
	// barWidth parameter is the total content width (label + bar space + percentage)
	// So total bar width = 2 (brackets) + barWidth + 1 (spacing) = 3 + barWidth
	coresPerLine := 4
	spacingBetweenBars := 1
	bracketOverhead := 2 // "[", "]"
	labelLen := 5         // "CPU00"
	percentLen := 6       // "100.0%"
	minBarSpace := 3      // Minimum space for the bar visualization
	
	// Minimum barWidth needed: label + minBarSpace + percentage
	minBarWidth := labelLen + minBarSpace + percentLen // 14
	
	// Calculate available width for bars (terminal width minus small margin)
	availableWidth := terminalWidth - 2
	
	// Calculate bar width: (availableWidth - coresPerLine * bracketOverhead - (coresPerLine - 1) * spacing) / coresPerLine
	barWidth := (availableWidth - coresPerLine*bracketOverhead - (coresPerLine-1)*spacingBetweenBars) / coresPerLine
	
	// Ensure minimum bar width (must fit label + bar space + percentage)
	if barWidth < minBarWidth {
		barWidth = minBarWidth
	}
	
	// If terminal is too narrow, reduce cores per line
	if barWidth < minBarWidth && coresPerLine > 2 {
		coresPerLine = 2
		barWidth = (availableWidth - coresPerLine*bracketOverhead - (coresPerLine-1)*spacingBetweenBars) / coresPerLine
		if barWidth < minBarWidth {
			barWidth = minBarWidth
		}
	}
	
	for i := 0; i < coreCount; i += coresPerLine {
		line := ""
		for j := 0; j < coresPerLine && i+j < coreCount; j++ {
			coreNum := i + j
			corePercent := stats.CPUCores[coreNum]
			coreColor := getColorForPercent(corePercent)
			coreLabel := fmt.Sprintf("CPU%02d", coreNum)
			corePercentText := fmt.Sprintf("%4.1f%%", corePercent)
			coreBarWithText := getPercentBarWithText(coreLabel, corePercentText, coreColor, corePercent, barWidth)
			if j < coresPerLine-1 {
				line += coreBarWithText + " "
			} else {
				line += coreBarWithText
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
