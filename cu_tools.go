package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// cleanSystemFull performs a full system cleanup, executing OS-specific tasks.
func cleanSystemFull() {
	// Define a cleanup task with a description and the command to execute
	type task struct {
		desc string   // Description of the task (for logging/spinner)
		cmd  []string // Command and arguments to run
	}

	var tasks []task

	switch goos {
	case "windows":
		// Windows cleanup tasks
		tasks = []task{
			{"Stopping Windows Update service", []string{"powershell", "Stop-Service", "-Name", "wuauserv", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Stopping BITS service", []string{"powershell", "Stop-Service", "-Name", "bits", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Prefetch", []string{"powershell", "Remove-Item", "C:\\Windows\\Prefetch\\*", "-Force", "-Recurse", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Error Reporting", []string{"powershell", "Remove-Item", "$env:ProgramData\\Microsoft\\Windows\\WER\\*", "-Recurse", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Windows Update Cache", []string{"powershell", "Remove-Item", "C:\\Windows\\SoftwareDistribution\\Download\\*", "-Recurse", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Delivery Optimization", []string{"powershell", "Remove-Item", "$env:SystemDrive\\ProgramData\\Microsoft\\Network\\Downloader\\*", "-Recurse", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Thumbnail Cache", []string{"powershell", "Remove-Item", "$env:LOCALAPPDATA\\Microsoft\\Windows\\Explorer\\thumbcache_*", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Cleaning Recycle Bin", []string{"powershell", "(New-Object -ComObject Shell.Application).NameSpace(10).Items() | ForEach-Object { Remove-Item $_.Path -Force -Recurse -ErrorAction SilentlyContinue }"}},
			{"Cleaning Temp Files", []string{"powershell", "-Command", "Get-ChildItem -Path $env:TEMP | ForEach-Object { Remove-Item $_.FullName -Recurse -Force -ErrorAction SilentlyContinue }"}},
			{"Cleaning Windows Temp Files", []string{"powershell", "Remove-Item", "$env:LOCALAPPDATA\\Microsoft\\Windows\\Caches\\*", "-Recurse", "-Force", "-ErrorAction", "SilentlyContinue"}},
			{"Flushing DNS Cache", []string{"ipconfig", "/flushdns"}},
		}

	default:
		// Linux / Unix-like cleanup tasks
		tasks = []task{
			{"Cleaning Thumbnail Cache", []string{"sh", "-c", "rm -rf ~/.cache/thumbnails/*"}},
			{"Cleaning System Logs >60 days", []string{"journalctl", "--vacuum-time=60d"}},
			{"Cleaning Trash", []string{"sh", "-c", "rm -rf ~/.local/share/Trash/*"}},
			{"Cleaning Temp Files", []string{"sh", "-c", "rm -rf /tmp/*"}},
			{"Cleaning Apt Cache", []string{"sudo", "apt-get", "clean"}},
			{"Cleaning Flatpak Cache", []string{"flatpak", "uninstall", "--unused", "-y"}},
			{"Cleaning Snap Cache", []string{"sudo", "rm", "-rf", "/var/cache/snapd/*"}},
			{"Cleaning DNF Cache", []string{"sh", "-c", "rm -rf /var/cache/dnf/*"}},
			{"Cleaning Pacman Cache", []string{"sh", "-c", "rm -rf /var/cache/pacman/pkg/*"}},
			{"Running Nix Garbage Collector", []string{"nix-collect-garbage", "-d"}},
		}
	}

	// Execute all tasks
	for _, t := range tasks {
		// Start spinner for the current task
		ctx, cancel := context.WithCancel(context.Background())
		go asyncSpinner(ctx, "Running: "+t.desc)
		time.Sleep(CMDWAIT)

		// Execute the task command and collect output
		output, err := runCommand(t.cmd)
		cancel() // Stop the spinner

		// Clear the spinner line and display results
		fmt.Printf("\r\033[2K") // ANSI code to clear current line
		if err != nil {
			// Task failed
			printError(t.desc + " failed")
			fmt.Printf("  Error: %s\n", err)
		} else {
			// Task succeeded
			printInfo(t.desc + " finished")
			if output != "" {
				// Print command output line by line
				lines := strings.Split(output, "\n")
				for _, line := range lines {
					fmt.Printf("  %s\n", line)
				}
			}
		}

		time.Sleep(200 * time.Millisecond) // Small pause before next task
	}
}

func CrunchySystemMonitor() {
	stop := make(chan struct{})

	go func() {
		bufio.NewScanner(os.Stdin).Scan()
		close(stop)
	}()

	createBar := func(value string) string {
		value = strings.TrimSuffix(value, "%")
		percent, err := strconv.Atoi(value)
		if err != nil {
			return "[??????????]"
		}
		totalBars := 10
		filled := percent * totalBars / 100
		if filled > totalBars {
			filled = totalBars
		}
		return "[" + strings.Repeat("█", filled) + strings.Repeat("░", totalBars-filled) + "]"
	}

	type DiskInfo struct {
		Name  string
		Total string
		Free  string
	}

	// Startwerte
	cpuCores := GetCPUCores()
	cpuUsage := getCPUUsagePercent()
	topCPU := getTopCPUProcesses()
	ramTotal := GetTotalRAM()
	freeRam := getFreeRAMPercent()
	diskName, diskTotal, diskFree := GetDiskInfo()

	for {
		select {
		case <-stop:
			printInfo("System Monitor stopped")
			return
		default:
			clearScreen()
			printCommandTitle("CrunchyUtils")
			printCommandTitle("System Monitor")
			line()

			// Anzeige direkt mit aktuellen Werten
			fmt.Printf("%s# CPU Info:%s\n", YELLOW, RC)
			fmt.Printf("└┬CPU Cores: %s\n", cpuCores)
			fmt.Printf(" └Usage    : %s %s\n", cpuUsage, createBar(cpuUsage))
			fmt.Printf(" %s# Top CPU Tasks:%s\n", YELLOW, RC)
			for i, task := range topCPU {
				prefix := "├"
				if i == len(topCPU)-1 {
					prefix = "└"
				}
				fmt.Printf(" %s %s\n", prefix, task)
			}
			line()

			fmt.Printf("%s# RAM Info:%s\n", YELLOW, RC)
			fmt.Printf("└┬Total RAM: %s\n", ramTotal)
			fmt.Printf(" └Free RAM : %s %s\n", freeRam, createBar(freeRam))
			line()

			diskPercent := "0%"
			totalGB, err1 := strconv.ParseFloat(strings.TrimSuffix(diskTotal, " GB"), 64)
			freeGB, err2 := strconv.ParseFloat(strings.TrimSuffix(diskFree, " GB"), 64)
			if err1 == nil && err2 == nil && totalGB > 0 {
				diskPercent = fmt.Sprintf("%d%%", int(freeGB*100/totalGB))
			}
			fmt.Printf("%s# Disk Info:%s\n", YELLOW, RC)
			fmt.Printf("└┬Disk Name : %s\n", diskName)
			fmt.Printf(" ├Total     : %s\n", diskTotal)
			fmt.Printf(" └Free      : %s %s %s\n", diskFree, diskPercent, createBar(diskPercent))
			line()

			fmt.Printf("Press [Enter] to stop the System Monitor\n")

			// Async Update für die nächste Runde
			go func() {
				cpuUsage = getCPUUsagePercent()
				topCPU = getTopCPUProcesses()
				freeRam = getFreeRAMPercent()
				diskName, diskTotal, diskFree = GetDiskInfo()
			}()

			time.Sleep(1 * time.Second)
		}
	}
}

// countdownTimer runs a timer counting down from totalSeconds
func countdownTimer(totalSeconds int) {
	printInfo("Timer started. Press [Enter] to cancel\n")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	stop := make(chan struct{})

	// Listen for Enter key to stop the timer
	go func() {
		bufio.NewScanner(os.Stdin).Scan()
		close(stop)
	}()

	for totalSeconds >= 0 {
		select {
		case <-ticker.C:
			// Display remaining time
			fmt.Printf("\r%s%s%s", GREEN, formatTime(totalSeconds), RC)
			totalSeconds--
		case <-stop:
			printInfo("Cancelled timer ")
			return
		}
	}

	printSuccess("Finished timer")

	// Show notification depending on OS
	switch goos {
	case "windows":
		cmd := exec.Command("powershell", "-Command", "Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('Finished timer', 'CrunchyUtils Timer')")
		cmd.Run()
	default:
		cmd := exec.Command("sh", "-c", "zenity --info --text=\"CrunchyUtils Finished timer!\"")
		cmd.Run()
	}
}

// stopwatch runs a simple stopwatch
func stopwatch() {
	start := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond) // update every half second
	defer ticker.Stop()

	stop := make(chan struct{})

	// Start listener for Enter key to stop the stopwatch
	printInfo("Stopwatch started. Press [Enter] to stop\n")
	go func() {
		bufio.NewScanner(os.Stdin).Scan()
		close(stop)
	}()

	for {
		select {
		case <-ticker.C:
			// Calculate elapsed time and display
			elapsed := int(time.Since(start).Seconds())
			fmt.Printf("\r%s%s%s", GREEN, formatTime(elapsed), RC)
		case <-stop:
			printInfo("Stopwatch stopped")
			return
		}
	}
}

// timerOrStopwatchMenu shows timer and stopwatch menu
func timerOrStopwatchMenu() {
	var cu_CmdName = "Timer/Stopwatch"
	for {
		printCommandTitle(cu_CmdName)
		fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
		fmt.Printf(" [1] - %sTimer%s\n", YELLOW, RC)
		fmt.Printf(" [2] - %sStopwatch%s\n", YELLOW, RC)
		line()
		fmt.Printf("Enter option%s", PROMPT)

		opt, _ := reader.ReadString('\n')
		opt = strings.TrimSpace(opt)

		switch opt {
		case "0":
			return
		case "1":
			fmt.Printf("Enter time (HH:MM:SS)%s", PROMPT)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			secs, err := parseTimeInput(input)
			if err != nil {
				fmt.Printf("Invalid time format\n")
				continue
			}
			countdownTimer(secs)
			pause()
			return
		case "2":
			stopwatch()
			pause()
			return
		default:
			fmt.Printf("Invalid Option\n")
			pause()
			return
		}
	}
}

// clipboardLogger continuously logs clipboard changes
func clipboardLogger() {
	var prev string
	stop := make(chan struct{})

	// Goroutine listens for Enter key to stop the logger
	go func() {
		printInfo("Clipboard logger started. Press [Enter] to cancel\n")
		bufio.NewScanner(os.Stdin).Scan()
		printInfo("Cancelled clipboard logger")
		close(stop)
	}()

	for {
		select {
		case <-stop:
			return // Exit main loop cleanly
		default:
			// Read current clipboard content depending on OS
			clip, err := func() (string, error) {
				switch runtime.GOOS {
				case "windows":
					// Windows: use PowerShell Get-Clipboard
					out, err := exec.Command("powershell", "-command", "Get-Clipboard").Output()
					return strings.TrimSpace(string(out)), err
				default:
					// Linux/Unix: try xclip or xsel
					var clip string
					var toolFound bool

					for _, tool := range []string{"xclip", "xsel"} {
						path, err := exec.LookPath(tool)
						if err != nil {
							continue
						}
						toolFound = true
						var cmd *exec.Cmd
						if tool == "xclip" {
							cmd = exec.Command(path, "-o", "-selection", "clipboard")
						} else {
							cmd = exec.Command(path, "--clipboard", "--output")
						}
						out, err := cmd.Output()
						if err == nil {
							clip = strings.TrimSpace(string(out))
							break
						}
					}

					if !toolFound {
						return "", fmt.Errorf("no clipboard tool found (xclip/xsel)")
					}
					return clip, nil
				}
			}()

			// Handle errors reading clipboard
			if err != nil {
				printError("Clipboard read failed:")
				fmt.Printf("%s", err)
				time.Sleep(2 * time.Second)
				continue
			}

			// Print new clipboard content if it has changed
			if clip != "" && clip != prev {
				prev = clip
				timestamp := time.Now().Format("2006-01-02 15:04:05")
				fmt.Printf("%s%s Copied:%s \"%s\"\n", YELLOW, timestamp, RC, clip)
			}

			time.Sleep(time.Second) // Poll clipboard every second
		}
	}
}

// powerTimer starts a shutdown or reboot timer and executes the action
func powerTimer(action, toption string) {
	// Ask user for timer duration
	fmt.Printf("Enter time for %s timer (HH:MM:SS)%s", action, PROMPT)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	secs, err := parseTimeInput(input)
	if err != nil {
		fmt.Printf("Invalid time format\n")
		pause()
		return
	}

	printInfo("Timer started. Press [Enter] to cancel\n")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	cancel := make(chan struct{})

	// Goroutine listens for Enter key to cancel
	go func() {
		bufio.NewScanner(os.Stdin).Scan()
		close(cancel)
	}()

	// Countdown loop
	for secs > 0 {
		select {
		case <-ticker.C:
			fmt.Printf("\r%sTime left: %s%s", GREEN, formatTime(secs), RC)
			secs--
		case <-cancel:
			printInfo("Cancelled shutdown/reboot timer")
			return
		}
	}

	printSuccess("Finished timer. Executing...\n")
	time.Sleep(2 * time.Second)

	// Determine the shutdown/reboot command based on OS
	var cmd []string
	switch goos {
	case "windows":
		switch toption {
		case "Wshutdown":
			cmd = []string{"shutdown", "/s", "/f", "/t", "0"}
		case "Wreboot":
			cmd = []string{"shutdown", "/r", "/f", "/t", "0"}
		}
	default: // linux/unix
		switch toption {
		case "Lshutdown":
			cmd = []string{"shutdown", "-h", "now"}
		case "Lreboot":
			cmd = []string{"shutdown", "-r", "now"}
		}
	}

	if len(cmd) > 0 {
		runCommand(cmd) // using the centralized helper
	}
}

func powerTimerMenu() {
	var cu_CmdName = "Shutdown/Reboot Timer"
	for {
		printCommandTitle(cu_CmdName)
		fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
		fmt.Printf(" [1] - %sShutdown Timer%s\n", YELLOW, RC)
		fmt.Printf(" [2] - %sReboot Timer%s\n", YELLOW, RC)
		line()
		fmt.Printf("Enter option%s", PROMPT)

		opt, _ := reader.ReadString('\n')
		opt = strings.TrimSpace(opt)

		switch opt {
		case "0":
			return
		case "1":
			var toption string
			if goos == "windows" {
				toption = "Wshutdown"
			} else {
				toption = "Lshutdown"
			}
			powerTimer("shutdown", toption)
			pause()
			return
		case "2":
			var toption string
			if goos == "windows" {
				toption = "Wreboot"
			} else {
				toption = "Lreboot"
			}
			powerTimer("reboot", toption)
			pause()
			return
		default:
			fmt.Printf("Invalid Option\n")
			pause()
			return
		}
	}
}

// weather fetches and prints weather info for a city
func weather() {
	fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
	fmt.Printf(" Enter city (e.g. Chicago)%s", PROMPT)
	cu_input, _ := reader.ReadString('\n')
	cu_input = strings.TrimSpace(cu_input)

	// Exit if user chooses 0
	if cu_input == "0" {
		return
	}

	// Require non-empty input
	if cu_input == "" {
		fmt.Printf("Please input a city\n")
		pause()
		return
	}

	// Start spinner while fetching weather
	ctx, cancel := context.WithCancel(context.Background())
	go asyncSpinner(ctx, "Weather...")

	// Build URL for wttr.in API
	url := fmt.Sprintf("http://wttr.in/%s?format=%%l:+%%C+%%t+%%w", cu_input)
	resp, err := http.Get(url)
	cancel() // stop spinner once request is done

	if err != nil {
		printError("Failed to get weather info")
		return
	}
	defer resp.Body.Close()

	// Clear line and set color
	fmt.Printf("\r\033[2K%s", YELLOW)

	// Print response line by line
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}
	fmt.Printf(RC) // reset color

	// Check for reading errors
	if err := scanner.Err(); err != nil {
		printError("Error reading weather data")
	}

	printSuccess("Finished getting weather infos from wttr.in")
	pause()
}

// infograb fetches and prints info about a domain/website
func infograb() {
	fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
	fmt.Printf(" Enter domain (e.g. google.com)%s", PROMPT)
	cu_input, _ := reader.ReadString('\n')
	cu_input = strings.TrimSpace(cu_input)

	// Exit if user chooses 0
	if cu_input == "0" {
		return
	}

	// Require non-empty input
	if cu_input == "" {
		fmt.Printf("Please input a domain\n")
		pause()
		return
	}

	// Start spinner while fetching info
	ctx, cancel := context.WithCancel(context.Background())
	go asyncSpinner(ctx, "Domain infos...")

	var output strings.Builder

	// Perform DNS lookup
	ips, err := net.LookupIP(cu_input)
	if err != nil {
		cancel()
		printError("Failed to lookup domain")
		return
	}
	output.WriteString(fmt.Sprintf("%s# Domain Info for %s:%s\n", YELLOW, cu_input, RC))
	output.WriteString(fmt.Sprintf("%s# IP Addresses:%s\n", YELLOW, RC))
	for _, ip := range ips {
		output.WriteString(fmt.Sprintf("  %s\n", ip.String()))
	}

	// Lookup Nameservers
	if ns, err := net.LookupNS(cu_input); err == nil {
		output.WriteString(fmt.Sprintf("%s# Nameservers:%s\n", YELLOW, RC))
		for _, n := range ns {
			output.WriteString(fmt.Sprintf("  %s\n", n.Host))
		}
	}

	// Lookup Mail Servers (MX records)
	if mx, err := net.LookupMX(cu_input); err == nil {
		output.WriteString(fmt.Sprintf("%s# Mail Servers (MX):%s\n", YELLOW, RC))
		for _, m := range mx {
			output.WriteString(fmt.Sprintf("  %s (Pref %d)\n", m.Host, m.Pref))
		}
	}

	// Fetch HTTP headers
	if resp, err := http.Head("http://" + cu_input); err == nil {
		output.WriteString(fmt.Sprintf("%s# HTTP Headers:%s\n", YELLOW, RC))
		for k, v := range resp.Header {
			output.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(v, ", ")))
		}
		resp.Body.Close()
	}

	// Fetch HTTPS headers
	if resp, err := http.Head("https://" + cu_input); err == nil {
		output.WriteString(fmt.Sprintf("%s# HTTPS Headers:%s\n", YELLOW, RC))
		for k, v := range resp.Header {
			output.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(v, ", ")))
		}
		resp.Body.Close()
	}

	// Stop spinner and print all collected info
	cancel()
	fmt.Printf("\r\033[2K%s%s", output.String(), RC)
	printSuccess("Finished getting infos")
	pause()
}
