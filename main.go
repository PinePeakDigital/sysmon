package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
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

	// Main stats bars with labels overlaid
	barWidth := m.width - 2 // Full width minus small margin
	if barWidth < 20 {
		barWidth = 20
	}

	// CPU
	cpuStyle := getColorStyle(m.stats.CPUUsage)
	cpuLabel := "CPU Usage"
	cpuPercent := fmt.Sprintf("%5.1f%%", m.stats.CPUUsage)
	cpuBar := createBarWithText(cpuLabel, cpuPercent, m.stats.CPUUsage, barWidth, cpuStyle)
	s.WriteString(cpuBar + "\n")

	// GPU
	gpuStyle := getColorStyle(m.stats.GPUUsage)
	gpuLabel := "GPU Usage"
	gpuPercent := fmt.Sprintf("%3.0f%%", m.stats.GPUUsage)
	gpuBar := createBarWithText(gpuLabel, gpuPercent, m.stats.GPUUsage, barWidth, gpuStyle)
	s.WriteString(gpuBar + "\n")

	// Memory
	memStyle := getColorStyle(m.stats.MemoryUsage)
	memLabel := "Memory"
	memPercent := fmt.Sprintf("%5.1f%%", m.stats.MemoryUsage)
	memBar := createBarWithText(memLabel, memPercent, m.stats.MemoryUsage, barWidth, memStyle)
	s.WriteString(memBar + "\n")

	// GPU Memory
	gpuMemStyle := getColorStyle(m.stats.GPUMemory)
	gpuMemLabel := "GPU Memory"
	gpuMemPercent := fmt.Sprintf("%4.1f%%", m.stats.GPUMemory)
	gpuMemBar := createBarWithText(gpuMemLabel, gpuMemPercent, m.stats.GPUMemory, barWidth, gpuMemStyle)
	s.WriteString(gpuMemBar + "\n")

	s.WriteString("\n")

	// CPU cores with labels overlaid
	coreCount := len(m.stats.CPUCores)
	coresPerLine := 4
	spacingBetweenBars := 2

	availableWidth := m.width - 2
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

			// Create bar with label and percentage overlaid
			coreBar := createBarWithText(coreLabel, corePercentText, corePercent, coreBarWidth, coreStyle)

			if j < coresPerLine-1 {
				line.WriteString(coreBar + "  ")
			} else {
				line.WriteString(coreBar)
			}
		}
		s.WriteString(line.String() + "\n")
	}

	s.WriteString("\n")

	// Process list header
	headerStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	s.WriteString(headerStyle.Render(fmt.Sprintf("%-10s %5s  %5s  %s", "PID", "CPU%", "MEM%", "COMMAND")))
	s.WriteString("\n")

	// Process list
	for _, proc := range m.stats.Processes {
		cpuStyle := getColorStyle(proc.CPU)
		memStyle := getColorStyle(float64(proc.Memory))
		s.WriteString(fmt.Sprintf("%-10d %s  %s  %s\n",
			proc.PID,
			cpuStyle.Render(fmt.Sprintf("%5.1f", proc.CPU)),
			memStyle.Render(fmt.Sprintf("%5.1f", proc.Memory)),
			proc.Command))
	}

	return s.String()
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
	// We'll use the same color for background
	fgColor := style.GetForeground()
	bgStyle := lipgloss.NewStyle().Background(fgColor).Foreground(lipgloss.Color("0")) // Black text on colored background

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
				result.WriteString(" ")
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

	// Get per-core CPU usage
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

	if len(procInfos) > 10 {
		procInfos = procInfos[:10]
	}

	return procInfos
}
