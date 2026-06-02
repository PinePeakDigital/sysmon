package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	// UnifiedMemory is true on systems where the CPU and GPU share a single
	// memory pool (e.g. Apple Silicon). When set, the GPU has no dedicated
	// VRAM and a separate "GPU Memory" reading would be redundant.
	UnifiedMemory bool
}

type ProcessInfo struct {
	PID     int32
	CPU     float64
	Memory  float32
	Command string
}

type model struct {
	stats  SystemStats
	width  int
	height int
}

type tickMsg struct{}

// Constants for process list formatting
const (
	// Width of fixed columns in the process list based on "%-10d %s  %s  %s\n":
	// PID (10) + space (1) + CPU% (6) + spaces (2) + MEM% (5) + spaces (2) = 26
	fixedColumnsWidth = 26
	// Minimum width for the COMMAND column to show something useful
	minCommandWidth = 10
)

// GPU vendor type
type gpuVendor int

const (
	gpuVendorNone gpuVendor = iota
	gpuVendorNVIDIA
	gpuVendorAMD
	// gpuVendorApple is the integrated GPU on Apple Silicon Macs, which
	// shares a single unified memory pool with the CPU.
	gpuVendorApple
)

// Cache for detected GPU vendor to avoid repeated command execution
var detectedGPUVendor gpuVendor
var gpuVendorOnce sync.Once

func main() {
	// Detect GPU vendor once at startup
	detectGPUVendor()

	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}

// detectGPUVendor detects which GPU vendor tools are available and caches the result
func detectGPUVendor() {
	gpuVendorOnce.Do(func() {
		// Try NVIDIA first
		cmd := exec.Command("nvidia-smi", "--query-gpu=utilization.gpu", "--format=csv,noheader,nounits")
		if err := cmd.Run(); err == nil {
			detectedGPUVendor = gpuVendorNVIDIA
			return
		}

		// Try AMD
		cmd = exec.Command("rocm-smi", "--showuse")
		if err := cmd.Run(); err == nil {
			detectedGPUVendor = gpuVendorAMD
			return
		}

		// Try Apple Silicon's integrated GPU. ioreg exposes GPU utilization via
		// the IOAccelerator class without requiring sudo. Gate on arm64: Intel
		// Macs also expose IOAccelerator stats but have discrete/non-unified
		// memory, so the Apple parser and unified-memory layout don't apply.
		if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
			cmd = exec.Command("ioreg", "-r", "-d", "1", "-w", "0", "-c", "IOAccelerator")
			if output, err := cmd.Output(); err == nil && strings.Contains(string(output), "Device Utilization %") {
				detectedGPUVendor = gpuVendorApple
				return
			}
		}

		// No GPU tools available
		detectedGPUVendor = gpuVendorNone
	})
}

func initialModel() model {
	return model{
		stats: collectStats(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), updateStats())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c", "esc":
			return m, tea.Quit
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(tick(), updateStats())

	case SystemStats:
		m.stats = msg
		return m, nil

	default:
		return m, nil
	}
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var s strings.Builder

	// Define styles
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

	// Helper function to get color style based on percentage
	getColorStyle := func(percent float64) lipgloss.Style {
		if percent < 50.0 {
			return greenStyle
		} else if percent < 80.0 {
			return yellowStyle
		}
		return redStyle
	}

	// Main stats bars with labels overlaid in a 2x2 grid
	// Calculate bar width for 2 bars per line with spacing
	spacingBetweenBars := 2
	availableWidth := m.width
	barWidth := (availableWidth - spacingBetweenBars) / 2
	if barWidth < 20 {
		barWidth = 20
	}

	// Row 1: CPU Usage | GPU Usage
	cpuStyle := getColorStyle(m.stats.CPUUsage).Underline(true)
	cpuLabel := "CPU Usage"
	cpuPercent := fmt.Sprintf("%5.1f%%", m.stats.CPUUsage)
	cpuBar := createBarWithText(cpuLabel, cpuPercent, m.stats.CPUUsage, barWidth, cpuStyle)

	gpuStyle := getColorStyle(m.stats.GPUUsage).Underline(true)
	gpuLabel := "GPU Usage"
	gpuPercent := fmt.Sprintf("%3.0f%%", m.stats.GPUUsage)
	gpuBar := createBarWithText(gpuLabel, gpuPercent, m.stats.GPUUsage, barWidth, gpuStyle)

	s.WriteString(cpuBar + "  " + gpuBar + "\n")

	// Row 2: Memory | GPU Memory.
	// On unified-memory systems (Apple Silicon) the CPU and GPU share one
	// memory pool, so we show a single full-width "Unified Memory" bar instead
	// of a redundant CPU/GPU memory split.
	memStyle := getColorStyle(m.stats.MemoryUsage).Underline(true)
	memPercent := fmt.Sprintf("%5.1f%%", m.stats.MemoryUsage)

	if m.stats.UnifiedMemory {
		// Span the same total width the two side-by-side bars would occupy so
		// the bar's right edge aligns with the CPU/GPU row above it.
		unifiedWidth := barWidth*2 + spacingBetweenBars
		memBar := createBarWithText("Unified Memory", memPercent, m.stats.MemoryUsage, unifiedWidth, memStyle)
		s.WriteString(memBar + "\n")
	} else {
		memBar := createBarWithText("Memory", memPercent, m.stats.MemoryUsage, barWidth, memStyle)

		gpuMemStyle := getColorStyle(m.stats.GPUMemory).Underline(true)
		gpuMemPercent := fmt.Sprintf("%4.1f%%", m.stats.GPUMemory)
		gpuMemBar := createBarWithText("GPU Memory", gpuMemPercent, m.stats.GPUMemory, barWidth, gpuMemStyle)

		s.WriteString(memBar + "  " + gpuMemBar + "\n")
	}

	s.WriteString("\n")

	// CPU cores with labels overlaid
	coreCount := len(m.stats.CPUCores)
	coresPerLine := 4
	spacingBetweenBars = 2

	availableWidth = m.width
	// Each bar needs space for label (5 chars) + percentage (6 chars) + some bar space
	// Total overhead is just spacing between bars since label/percent are inside
	coreBarWidth := (availableWidth - (coresPerLine-1)*spacingBetweenBars) / coresPerLine

	// Ensure minimum bar width (must fit label + percentage + some bar space)
	minBarWidth := 15 // "CPU00" (5) + " 100.0%" (7) + 3 bar space
	if coreBarWidth < minBarWidth {
		coreBarWidth = minBarWidth
	}

	if coreBarWidth < minBarWidth && coresPerLine > 2 {
		coresPerLine = 2
		coreBarWidth = (availableWidth - (coresPerLine-1)*spacingBetweenBars) / coresPerLine
		if coreBarWidth < minBarWidth {
			coreBarWidth = minBarWidth
		}
	}

	for i := 0; i < coreCount; i += coresPerLine {
		var line strings.Builder
		for j := 0; j < coresPerLine && i+j < coreCount; j++ {
			coreNum := i + j
			corePercent := m.stats.CPUCores[coreNum]
			coreStyle := getColorStyle(corePercent)
			coreLabel := fmt.Sprintf("CPU%02d", coreNum)
			corePercentText := fmt.Sprintf("%4.1f%%", corePercent)

			// Create bar with label and percentage overlaid (with underline)
			coreStyleUnderlined := coreStyle.Underline(true)
			coreBar := createBarWithText(coreLabel, corePercentText, corePercent, coreBarWidth, coreStyleUnderlined)

			if j < coresPerLine-1 {
				line.WriteString(coreBar + "  ")
			} else {
				line.WriteString(coreBar)
			}
		}
		s.WriteString(line.String() + "\n")
	}

	s.WriteString("\n")

	// Calculate how many lines we've used so far
	// 2 lines for main stats bars + 1 blank + CPU cores lines + 1 blank + 1 header = 5 + CPU core lines
	coreLines := (coreCount + coresPerLine - 1) / coresPerLine // Ceiling division
	linesUsed := 2 + 1 + coreLines + 1 + 1                     // stats + blank + cores + blank + header

	// Calculate available lines for processes (leave 1 line margin at bottom)
	// If height is 0 or not set, use a reasonable default (24 lines is common)
	terminalHeight := m.height
	if terminalHeight == 0 {
		terminalHeight = 24 // Default terminal height
	}

	availableLines := terminalHeight - linesUsed - 1
	if availableLines < 1 {
		availableLines = 1 // Always show at least 1 process
	}

	// Limit number of processes to show
	maxProcesses := availableLines
	if maxProcesses > len(m.stats.Processes) {
		maxProcesses = len(m.stats.Processes)
	}

	// Process list header
	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-10s %6s  %5s  %s", "PID", "CPU%", "MEM%", "COMMAND")))
	s.WriteString("\n")

	// Process list (no underline for percentages)
	// Calculate available width for COMMAND column
	commandWidth := m.width - fixedColumnsWidth
	if commandWidth < minCommandWidth {
		commandWidth = minCommandWidth
	}

	for i := 0; i < maxProcesses; i++ {
		proc := m.stats.Processes[i]
		cpuStyle := getColorStyle(proc.CPU).Underline(false)
		memStyle := getColorStyle(float64(proc.Memory)).Underline(false)

		// Truncate command from the left if it's too long
		truncatedCommand := truncateLeft(proc.Command, commandWidth)

		s.WriteString(fmt.Sprintf("%-10d %s  %s  %s\n",
			proc.PID,
			cpuStyle.Render(fmt.Sprintf("%6.1f", proc.CPU)),
			memStyle.Render(fmt.Sprintf("%5.1f", proc.Memory)),
			truncatedCommand))
	}

	return s.String()
}

// truncateLeft truncates a string from the left if it exceeds maxWidth,
// adding "..." prefix to indicate truncation
func truncateLeft(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	// Convert to runes to handle unicode characters correctly
	runes := []rune(s)

	if len(runes) <= maxWidth {
		return s
	}

	// Need room for "..." prefix (3 characters)
	if maxWidth <= 3 {
		return "..."[:maxWidth]
	}

	// Use strings.Builder for efficient string construction
	var builder strings.Builder
	builder.Grow(maxWidth) // Pre-allocate capacity
	builder.WriteString("...")
	builder.WriteString(string(runes[len(runes)-(maxWidth-3):]))
	return builder.String()
}

func createSimpleBar(percent float64, width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}

	filled := int((percent / 100.0) * float64(width))
	bar := strings.Builder{}

	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(style.Render("█"))
		} else {
			bar.WriteString("░")
		}
	}

	return bar.String()
}

// createBarWithText creates a bar with label and percentage overlaid using Lipgloss background colors
func createBarWithText(label, percentText string, percent float64, width int, style lipgloss.Style) string {
	if width <= 0 {
		return label + " " + percentText
	}

	if percent < 0 {
		percent = 0
	} else if percent > 100 {
		percent = 100
	}

	filled := int((percent / 100.0) * float64(width))
	labelRunes := []rune(label)
	percentRunes := []rune(percentText)
	labelLen := len(labelRunes)
	percentLen := len(percentRunes)
	totalTextLen := labelLen + percentLen

	// If text is longer than bar width, just return text
	if totalTextLen >= width {
		return style.Render(label + " " + percentText)
	}

	// Get the foreground color and create a background style
	// We'll use the same color for background, and preserve underline
	fgColor := style.GetForeground()
	bgStyle := lipgloss.NewStyle().Background(fgColor).Foreground(lipgloss.Color("0")) // Black text on colored background
	if style.GetUnderline() {
		bgStyle = bgStyle.Underline(true)
	}

	// Calculate where percentage starts (right-aligned)
	percentStart := width - percentLen
	result := strings.Builder{}

	// Build bar with text overlaid
	for i := 0; i < width; i++ {
		if i < labelLen {
			// Label portion (left-aligned)
			if i < filled {
				// Label on filled portion - use background color with inverse text
				result.WriteString(bgStyle.Render(string(labelRunes[i])))
			} else {
				// Label on unfilled portion - use foreground color
				result.WriteString(style.Render(string(labelRunes[i])))
			}
		} else if i < percentStart {
			// Middle portion (bar only)
			if i < filled {
				result.WriteString(bgStyle.Render(" "))
			} else {
				// Apply underline to unfilled spaces too
				result.WriteString(style.Render(" "))
			}
		} else {
			// Percentage portion (right-aligned)
			percentIdx := i - percentStart
			if i < filled {
				// Percentage on filled portion - use background color with inverse text
				result.WriteString(bgStyle.Render(string(percentRunes[percentIdx])))
			} else {
				// Percentage on unfilled portion - use foreground color
				result.WriteString(style.Render(string(percentRunes[percentIdx])))
			}
		}
	}

	return result.String()
}

func tick() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func updateStats() tea.Cmd {
	return func() tea.Msg {
		return collectStats()
	}
}

func collectStats() SystemStats {
	stats := SystemStats{}

	// Get per-core CPU usage. On some platforms this can fail or return nothing
	// (notably macOS binaries built without cgo, where gopsutil cannot read
	// per-core stats). Don't silently swallow that: fall back to the aggregate
	// reading so the overall CPU bar still works instead of showing 0%.
	perCoreCPU, err := cpu.Percent(time.Second, true)
	if err == nil && len(perCoreCPU) > 0 {
		stats.CPUCores = perCoreCPU

		// Calculate average CPU usage from per-core data
		var sum float64
		for _, val := range perCoreCPU {
			sum += val
		}
		stats.CPUUsage = sum / float64(len(perCoreCPU))
	} else if total, terr := cpu.Percent(time.Second, false); terr == nil && len(total) > 0 {
		// Per-core data unavailable; show aggregate usage with no per-core bars.
		stats.CPUUsage = total[0]
	}

	// Memory Usage
	memInfo, _ := mem.VirtualMemory()
	if memInfo != nil {
		stats.MemoryUsage = memInfo.UsedPercent
	}

	// GPU stats
	stats.GPUUsage = getGPUUsage()
	stats.GPUMemory = getGPUMemory()
	// On unified-memory systems (Apple Silicon) the GPU shares the system
	// memory pool, so there is no separate VRAM figure to report.
	stats.UnifiedMemory = detectedGPUVendor == gpuVendorApple

	// Process list
	stats.Processes = getTopProcesses()

	return stats
}

func getGPUUsage() float64 {
	switch detectedGPUVendor {
	case gpuVendorNVIDIA:
		return getGPUUsageNVIDIA()
	case gpuVendorAMD:
		return getGPUUsageAMD()
	case gpuVendorApple:
		return getGPUUsageApple()
	default:
		return 0.0
	}
}

func getGPUUsageApple() float64 {
	cmd := exec.Command("ioreg", "-r", "-d", "1", "-w", "0", "-c", "IOAccelerator")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}
	return parseAppleGPUUsage(string(output))
}

// parseAppleGPUUsage extracts the "Device Utilization %" value from ioreg
// IOAccelerator output. This is the same figure Activity Monitor reports as
// GPU usage. Returns 0.0 if the value can't be found or parsed.
func parseAppleGPUUsage(output string) float64 {
	const key = `"Device Utilization %"=`
	idx := strings.Index(output, key)
	if idx == -1 {
		return 0.0
	}

	rest := output[idx+len(key):]
	end := 0
	// ioreg reports this value as an integer today, but accept a decimal point
	// too so a fractional reading wouldn't be silently truncated.
	for end < len(rest) && ((rest[end] >= '0' && rest[end] <= '9') || rest[end] == '.') {
		end++
	}
	if end == 0 {
		return 0.0
	}

	usage, err := strconv.ParseFloat(rest[:end], 64)
	if err != nil {
		return 0.0
	}
	return usage
}

func getGPUUsageNVIDIA() float64 {
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

func getGPUUsageAMD() float64 {
	cmd := exec.Command("rocm-smi", "--showuse")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	// Parse rocm-smi output
	// rocm-smi --showuse output format:
	// ========================= ROCm System Management Interface =========================
	// ================================ GPU use ================================
	// GPU[0]		: GPU use (%): 25
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Look for GPU[0] specifically at the start and check for "GPU use (%)"
		if strings.HasPrefix(strings.TrimSpace(line), "GPU[0]") && strings.Contains(line, "GPU use (%)") {
			// Extract value after the last colon
			if valueStr, ok := extractValueAfterLastColon(line); ok {
				if usage, err := strconv.ParseFloat(valueStr, 64); err == nil {
					return usage
				}
			}
		}
	}

	return 0.0
}

// extractValueAfterLastColon extracts and trims the string after the last colon in a line
func extractValueAfterLastColon(line string) (string, bool) {
	lastColonIdx := strings.LastIndex(line, ":")
	if lastColonIdx == -1 || lastColonIdx+1 > len(line) {
		return "", false
	}
	return strings.TrimSpace(line[lastColonIdx+1:]), true
}

func getGPUMemory() float64 {
	switch detectedGPUVendor {
	case gpuVendorNVIDIA:
		return getGPUMemoryNVIDIA()
	case gpuVendorAMD:
		return getGPUMemoryAMD()
	default:
		return 0.0
	}
}

func getGPUMemoryNVIDIA() float64 {
	cmd := exec.Command("nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

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

func getGPUMemoryAMD() float64 {
	cmd := exec.Command("rocm-smi", "--showmeminfo", "vram")
	output, err := cmd.Output()
	if err != nil {
		return 0.0
	}

	// Parse rocm-smi output
	// rocm-smi --showmeminfo vram output format:
	// ========================= ROCm System Management Interface =========================
	// ================================ VRAM Total Memory (B) ================================
	// GPU[0]		: VRAM Total Memory (B): 17163091968
	// ================================ VRAM Total Used Memory (B) ================================
	// GPU[0]		: VRAM Total Used Memory (B): 1234567890
	var totalMem, usedMem float64
	var foundTotal, foundUsed bool

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Look for GPU[0] specifically at the start
		if strings.HasPrefix(trimmedLine, "GPU[0]") {
			if strings.Contains(line, "VRAM Total Memory (B)") && !strings.Contains(line, "Used") {
				// Extract value after the last colon
				if totalStr, ok := extractValueAfterLastColon(line); ok {
					if total, err := strconv.ParseFloat(totalStr, 64); err == nil {
						totalMem = total
						foundTotal = true
						// If we've found both values, we can stop searching
						if foundUsed {
							break
						}
					}
				}
			} else if strings.Contains(line, "VRAM Total Used Memory (B)") {
				// Extract value after the last colon
				if usedStr, ok := extractValueAfterLastColon(line); ok {
					if used, err := strconv.ParseFloat(usedStr, 64); err == nil {
						usedMem = used
						foundUsed = true
						// If we've found both values, we can stop searching
						if foundTotal {
							break
						}
					}
				}
			}
		}
	}

	// Only calculate percentage if we successfully parsed both values
	if foundTotal && foundUsed && totalMem > 0 {
		return (usedMem / totalMem) * 100.0
	}

	return 0.0
}

func getTopProcesses() []ProcessInfo {
	processes, _ := process.Processes()
	var procInfos []ProcessInfo

	for _, p := range processes {
		cpuPercent, err := p.CPUPercent()
		if err != nil {
			continue
		}

		memPercent, err := p.MemoryPercent()
		if err != nil {
			continue
		}

		if cpuPercent == 0 {
			continue
		}

		exe, err := p.Exe()
		if err != nil {
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

	sort.Slice(procInfos, func(i, j int) bool {
		return procInfos[i].CPU > procInfos[j].CPU
	})

	// Return up to 100 processes (enough for most terminal sizes)
	// The view will limit further based on available height
	maxToCollect := 100
	if len(procInfos) > maxToCollect {
		procInfos = procInfos[:maxToCollect]
	}

	return procInfos
}
