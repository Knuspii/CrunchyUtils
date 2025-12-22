package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gen2brain/beeep"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

func printInfo(msg string) {
	fmt.Printf("%s[INFO] %s%s\n", YELLOW, RC, msg)
}
func printError(msg string) {
	fmt.Printf("%s[ERROR] %s%s\n", RED, RC, msg)
}
func printSuccess(msg string) {
	fmt.Printf("\n%s[INFO] %s%s\n", GREEN, RC, msg)
}

func line() {
	fmt.Printf("%s#%s#%s\n", YELLOW, strings.Repeat("═", COLS-2), RC)
}
func cmdline() {
	fmt.Printf("%s#%s#%s\n", RED, strings.Repeat("═", COLS-2), RC)
}

// Pause waits for enter
func pause() {
	fmt.Printf("\nPress [Enter] to continue: ")
	reader.ReadString('\n')
}

// yesNo asks question, returns true if yes
func yesNo(question string) bool {
	for {
		fmt.Printf("%s (yes/no)%s", question, PROMPT)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		switch answer {
		case "y", "yes", "ye", "yup", "ja", "yessir", "yep":
			return true
		case "n", "no", "nah", "ne", "nein", "na", "nope":
			return false
		}
	}
}

func printCommandTitle(name string) {
	title := fmt.Sprintf("#   %s   #", name)
	padding := strings.Repeat(" ", (COLS-len(title))/2)
	fmt.Printf("%s%s%s%s\n", CYAN, padding, title, RC)
}

// asyncSpinner displays a spinning "loading" animation in the terminal.
// It runs asynchronously and stops when the provided context is canceled.
func asyncSpinner(ctx context.Context, text string) {
	i := 0 // Index for spinner frames

	for {
		select {
		// If the context is canceled, stop the spinner and return
		case <-ctx.Done():
			return

		// Default case: continue spinning
		default:
			// Print spinner line:
			// \r       -> Carriage return to overwrite the same line
			// [LOADING] -> Static label
			// YELLOW/RC -> Apply color and reset
			// text      -> Custom text passed to the spinner
			// SPINNERFRAMES[i%len(SPINNERFRAMES)] -> Rotate through spinner characters
			fmt.Printf("\r%s[LOADING]%s %s %c  ", YELLOW, RC, text, SPINNERFRAMES[i%len(SPINNERFRAMES)])

			time.Sleep(100 * time.Millisecond) // Wait a short time before next frame
			i++                                // Move to the next spinner frame
		}
	}
}

// runCommand executes an external command and returns its combined output (stdout + stderr).
// It takes a slice of strings, where the first element is the command and the rest are arguments.
func runCommand(cmd []string) (string, error) {
	// Check if the command slice is empty
	if len(cmd) == 0 {
		return "", errors.New("command is empty")
	}

	// Create an exec.Command object with the command and its arguments
	c := exec.Command(cmd[0], cmd[1:]...)

	// Run the command and capture both stdout and stderr
	outBytes, err := c.CombinedOutput()

	// Convert output bytes to string and trim whitespace/newlines
	out := strings.TrimSpace(string(outBytes))

	// If the command failed, return the output along with a formatted error
	if err != nil {
		return out, fmt.Errorf(
			"command '%s' failed: %v\nOutput: %s",
			strings.Join(cmd, " "), // Reconstruct the command for the error message
			err,
			out,
		)
	}

	// If successful, return the output and nil error
	return out, nil
}

func notifyAlarm() {
	// system notification
	beeep.Alert(
		"CrunchyUtils",
		"Alert!",
		"",
	)
	beeep.Beep(600, 150)
	time.Sleep(100 * time.Millisecond)
	beeep.Beep(800, 200)
	time.Sleep(100 * time.Millisecond)
	beeep.Beep(1000, 300)
}

func clearScreen() {
	var cmd *exec.Cmd
	if goos == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Fallback: ANSI for Clear + Reset Cursor
		fmt.Print("\033[H\033[2J")
	}
}

// formatTime converts a duration in seconds into a human-readable HH:MM:SS string.
func formatTime(seconds int) string {
	h := seconds / 3600                           // Calculate hours
	m := (seconds % 3600) / 60                    // Calculate remaining minutes
	s := seconds % 60                             // Calculate remaining seconds
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s) // Format as HH:MM:SS with leading zeros
}

// parseTimeInput converts a time string in the format "HH:MM:SS" into total seconds.
// Returns an error if the format is invalid or the total time is non-positive.
func parseTimeInput(input string) (int, error) {
	// Split input string by colon
	parts := strings.Split(input, ":")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid time format") // Must be HH:MM:SS
	}

	// Parse hours
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}

	// Parse minutes
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	// Parse seconds
	s, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, err
	}

	// Calculate total seconds
	total := h*3600 + m*60 + s

	// Ensure total time is positive
	if total <= 0 {
		return 0, fmt.Errorf("time must be positive")
	}

	return total, nil
}

func getCPUUsagePercent() string {
	// CPU Prozent über 500ms messen
	percentages, err := cpu.Percent(500*time.Millisecond, false)
	if err != nil || len(percentages) == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.0f%%", percentages[0])
}

// GetCPUCores returns the number of logical CPU cores
func GetCPUCores() string {
	cores, err := cpu.Counts(true)
	if err != nil {
		return "Unknown"
	}
	return strconv.Itoa(cores)
}

// GetTotalRAM returns the total RAM in MB as a string
func GetTotalRAM() string {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return "Unknown"
	}
	return fmt.Sprintf("%.0f MB", float64(vm.Total)/1024/1024)
}

// GetDiskInfo returns disk mountpoint total space and used percentage
func GetDiskInfo() (string, string, string) {
	partitions, err := disk.Partitions(false)
	if err != nil || len(partitions) == 0 {
		return "Unknown", "0 GB", "0%"
	}

	root := partitions[0].Mountpoint
	usage, err := disk.Usage(root)
	if err != nil || usage.Total == 0 {
		return root, "0 GB", "0%"
	}

	total := fmt.Sprintf("%.0f GB", float64(usage.Total)/1024/1024/1024)
	usedPercent := fmt.Sprintf("%.0f%%", usage.UsedPercent)

	return root, total, usedPercent
}

// getCurrentPartitionUsedBytes returns used bytes of the current working directory's partition
func getCurrentPartitionUsedBytes() (uint64, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return 0, err
	}

	// Auf aktuelle Partition mounten
	partition := cwd
	if goos == "windows" {
		partition = filepath.VolumeName(cwd) + "\\"
	}

	usage, err := disk.Usage(partition)
	if err != nil {
		return 0, err
	}

	return usage.Used, nil
}

// getTopCPUProcesses returns the top 5 CPU-consuming processes on the system
func getTopCPUProcesses() []string {
	procs, err := process.Processes()
	if err != nil {
		return []string{"ERROR", "ERROR", "ERROR", "ERROR", "ERROR"}
	}

	type procCPU struct {
		name string
		cpu  float64
	}

	var list []procCPU

	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}

		cpu, err := p.CPUPercent()
		if err != nil || cpu <= 0 {
			continue
		}

		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "system") ||
			strings.HasPrefix(lower, "svchost") ||
			strings.HasPrefix(lower, "init") ||
			strings.HasPrefix(lower, "systemd") ||
			strings.HasPrefix(lower, "idle") ||
			strings.HasPrefix(lower, "cu_main") {
			continue
		}

		if len(name) > 18 {
			name = name[:15] + "..."
		}

		list = append(list, procCPU{name, cpu})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].cpu > list[j].cpu
	})

	var top []string
	for _, p := range list {
		dup := false
		for _, t := range top {
			if t == p.name {
				dup = true
				break
			}
		}
		if dup {
			continue
		}

		top = append(top, p.name)
		if len(top) == 5 {
			break
		}
	}

	for len(top) < 5 {
		top = append(top, "...")
	}

	return top
}

// getRAMUsagePercent returns the percentage of used RAM as a string (e.g. "58%")
func getRAMUsagePercent() string {
	vm, err := mem.VirtualMemory()
	if err != nil || vm.Total == 0 {
		return "0%"
	}

	usedPercent := (float64(vm.Used) / float64(vm.Total)) * 100
	return fmt.Sprintf("%.0f%%", usedPercent)
}

// getUptime returns the system uptime as a formatted string, e.g., "12H:34M".
// Supports Windows and Linux/Unix systems.
func getUptime() string {
	sec, err := host.Uptime()
	if err != nil {
		return "unknown"
	}

	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) - h*60

	if h > 999 {
		return "+999H"
	}

	return fmt.Sprintf("%dH:%02dM", h, m)
}
