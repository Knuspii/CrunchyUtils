// ##################################
// CrunchyUtils
// Author: Knuspii (M)
//
// Description:
// CrunchyUtils is a cross-platform terminal utility suite
// providing system monitoring, cleanup, timers, power tools,
// and various helper utilities for Windows and Linux.
//
// This file (cu_main.go) contains:
// - Program entry point
// - Menu system
// - UI rendering
// - Startup & initialization logic
//
// Warning:
// Code quality may cause emotional damage.
// ##################################

package main

import (
	"bufio"   // User input handling
	"context" // Goroutine lifecycle control
	"flag"    // CLI flags
	"fmt"     // Output formatting
	"os"      // OS interaction
	"os/exec" // Execute external commands
	"os/user" // Current user info
	"runtime" // OS detection
	"strings" // String utilities
	"time"    // Timing utilities

	"github.com/eiannone/keyboard" // Raw keyboard input
	"github.com/gen2brain/beeep"   // System beep / notifications
)

//
// ========================== CONSTANTS ==========================
//

const (
	CU_VERSION = "PRE 0.17"

	// Target terminal size
	COLS  = 70
	LINES = 27

	// Prompt styling
	PROMPT = (YELLOW + " >>:" + RC)

	// ANSI colors
	RED    = "\033[31m"
	YELLOW = "\033[33m"
	GREEN  = "\033[32m"
	BLUE   = "\033[34m"
	CYAN   = "\033[36m"
	RC     = "\033[0m"
)

//
// ========================== GLOBAL STATE ==========================
//

var (
	getcols, getlines int                           // Detected terminal size
	consoleRunning    = true                        // Main loop control
	goos              = runtime.GOOS                // Cached OS string
	reader            = bufio.NewReader(os.Stdin)   // Global input reader
	SPINNERFRAMES     = []rune{'|', '/', '-', '\\'} // Spinner animation frames
	CMDWAIT           = 1 * time.Second             // Artificial delay between commands
	// CLI flags
	Flagversion = flag.Bool("version", false, "Show version")
	Flagnoinit  = flag.Bool("no-init", false, "Dont resize window, etc.")
	Flagskip    = flag.Bool("skip", false, "Skip all delays")
	Flagnoadmin = flag.Bool("no-admin", false, "Skip admin/root request")
)

//
// ========================== ADMIN / ROOT HANDLING ==========================
//

// getAdmin makes sure the program runs with elevated privileges.
// Only re-launches itself if admin/root rights are missing.
func getAdmin() {
	switch goos {

	case "windows":
		// `net session` only succeeds when running as administrator
		// so failure = no admin rights
		cmd := exec.Command("net", "session")
		if err := cmd.Run(); err != nil {

			printInfo("Restarting as admin...\n")

			// Relaunch the current executable with PowerShell elevation
			elevate := exec.Command(
				"powershell",
				"-Command",
				"Start-Process",
				os.Args[0],
				"-Verb",
				"RunAs",
			)

			// Forward I/O so the new process behaves like the original
			elevate.Stdout = os.Stdout
			elevate.Stderr = os.Stderr

			if err := elevate.Run(); err != nil {
				// User denied UAC or PowerShell failed
				printError(fmt.Sprintf("Failed to restart as admin: %v", err))
				pause()
			} else {
				// Stop the non-admin instance
				os.Exit(0)
			}
		}

	default:
		// Unix systems: UID 0 == root
		if os.Geteuid() != 0 {

			printInfo("Requesting root privileges...\n")

			// Resolve absolute path of the running binary
			scriptPath, err := exec.LookPath(os.Args[0])
			if err != nil {
				printError(fmt.Sprintf("Executable not found: %v", err))
				return
			}

			// Preserve original CLI arguments
			args := append([]string{scriptPath}, os.Args[1:]...)

			// Relaunch via sudo
			cmd := exec.Command("sudo", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin

			if err := cmd.Run(); err != nil {
				// sudo failed or user aborted
				printError(fmt.Sprintf("Failed to restart with sudo: %v", err))
				pause()
			} else {
				// Kill the non-root process
				os.Exit(0)
			}
		}
	}
}

//
// ========================== STARTUP ==========================
//

// startup prints splash info and initializes the application
func startup() {
	printInfo("CrunchyUtils starting...")
	printInfo("CrunchyUtils " + CU_VERSION)
	fmt.Printf(" *******************\n")
	fmt.Printf(" *  CrunchyUtils   *\n")
	fmt.Printf(" *  c2025 Knuspii  *\n")
	fmt.Printf(" *******************\n")
	fmt.Printf("\n")
	switch goos {
	case "windows":
		fmt.Printf("%sWindows Detected%s\n", CYAN, RC)
	case "linux":
		fmt.Printf("%sLinux Detected%s\n", CYAN, RC)
	default:
		printError("OS: " + goos + " not supported!")
		pause()
		os.Exit(1)
	}
	now := time.Now()
	fmt.Printf("Time: %s //// Date: %s\n\n", now.Format("15:04:05"), now.Format("02.01.2006"))
	if !*Flagnoadmin {
		getAdmin()
	}
	time.Sleep(CMDWAIT)
	time.Sleep(CMDWAIT)
	if !*Flagnoinit {
		initApp()
	}
	fmt.Printf("\n%s**************************%s", GREEN, RC)
	printSuccess("LOADING SUCCESSFUL!")
	time.Sleep(CMDWAIT)
	time.Sleep(CMDWAIT)
	go beeep.Beep(600, 150)
}

//
// ========================== APPLICATION INIT ==========================
//

// initApp performs environment setup that depends on the terminal and OS
func initApp() {
	printInfo("Initializing...")
	// Resolve current user for display purposes
	usr, err := user.Current()
	if err != nil {
		printError("Username: unknown")
	} else {
		printInfo(fmt.Sprintf("Username: %s", usr.Username))
	}

	// Set terminal window title (works in most terminals)
	fmt.Printf("\033]0;CrunchyUtils\007")

	// Attempt to resize terminal to the expected layout
	// This is best-effort and highly terminal-dependent
	switch goos {
	case "windows":
		// Set CMD window title explicitly
		_, _ = runCommand([]string{"cmd", "/C", "title CrunchyUtils"})

		// Resize PowerShell window and buffer
		// Buffer height is larger to allow scrolling
		psCmd := fmt.Sprintf(
			`$Host.UI.RawUI.WindowSize = New-Object System.Management.Automation.Host.Size(%d,%d); `+
				`$Host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size(%d,300)`,
			COLS, LINES, COLS,
		)
		_, _ = runCommand([]string{"powershell", "-Command", psCmd})

	default:
		// ANSI escape resize (ignored by some terminals)
		fmt.Printf("\033[8;%d;%dt", LINES, COLS)
	}

	// ---- VERIFY SIZE ----
	// We re-detect terminal size to ensure layout assumptions are valid
	getcols, getlines = 0, 0
	var sizeErr error

	if goos == "windows" {
		// `mode con` prints current console dimensions
		out, err := runCommand([]string{"cmd", "/C", "mode con"})
		if err != nil {
			sizeErr = err
		} else {
			// Parse Columns / Lines from command output
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "Columns:") {
					fmt.Sscanf(line, "    Columns: %d", &getcols)
				}
				if strings.Contains(line, "Lines:") {
					fmt.Sscanf(line, "    Lines: %d", &getlines)
				}
			}
		}
	} else {
		// `stty size` returns rows cols from the active TTY
		out, err := runCommand([]string{"sh", "-c", "stty size < /dev/tty"})
		if err != nil {
			sizeErr = err
		} else {
			fmt.Sscanf(out, "%d %d", &getlines, &getcols)
		}
	}

	// Abort size validation if detection failed
	if sizeErr != nil || getcols == 0 || getlines == 0 {
		printError("Could not detect terminal size")
		time.Sleep(2 * time.Second)
		return
	}

	// Compare actual size with required layout size
	if getcols != COLS || getlines != LINES {
		printError(fmt.Sprintf("Terminal size mismatch got: %dx%d expected: %dx%d", getcols, getlines, COLS, LINES))
		time.Sleep(2 * time.Second)
	} else {
		printInfo(fmt.Sprintf("Terminal size OK: %dx%d", getcols, getlines))
	}
}

func timerMenu() {
	for {
		printCommandTitle("Timer/Stopwatch")
		fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
		fmt.Printf(" [1] - %sTimer%s\n", YELLOW, RC)
		fmt.Printf(" [2] - %sStopwatch%s\n", YELLOW, RC)
		line()
		fmt.Printf("Press key (0-2)\n")

		c, _, err := keyboard.GetSingleKey()
		if err != nil {
			continue
		}

		switch c {
		case '0', 27: // Return or ESC
			return
		case '1':
			fmt.Printf("Enter time (HH:MM:SS)%s", PROMPT)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			secs, err := parseTimeInput(input)
			if err != nil {
				fmt.Printf("\nInvalid time format\n")
				pause()
				continue
			}
			countdownTimer(secs)
		case '2':
			stopwatch()
		default:
			fmt.Printf("\nInvalid Option\n")
			time.Sleep(2 * time.Second)
		}

		pause()
	}
}

func powerMenu() {
	for {
		printCommandTitle("Shutdown/Reboot Timer")
		fmt.Printf(" [0] - %sReturn%s\n", RED, RC)
		fmt.Printf(" [1] - %sShutdown Timer%s\n", YELLOW, RC)
		fmt.Printf(" [2] - %sReboot Timer%s\n", YELLOW, RC)
		line()
		fmt.Printf("Press key (0-2)\n")

		c, _, err := keyboard.GetSingleKey()
		if err != nil {
			continue
		}

		var toption string
		switch c {
		case '0', 27: // Return or ESC
			return
		case '1':
			if goos == "windows" {
				toption = "Wshutdown"
			} else {
				toption = "Lshutdown"
			}
			powerTimer("shutdown/reboot", toption)
		case '2':
			if goos == "windows" {
				toption = "Wreboot"
			} else {
				toption = "Lreboot"
			}
			powerTimer("shutdown/reboot", toption)
		default:
			fmt.Printf("\nInvalid Option\n")
			time.Sleep(2 * time.Second)
		}

		pause()
	}
}

func crunchytext() {
	if getcols > COLS+6 {
		fmt.Printf(`%s██      ███       ███  ████  ██   ███  ███      ███  ████  ██  ████  █▓▓▓▒▒▒
▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓    ▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓  ▓▓  ▓▓▓▓ ▓▓ ▒
▒  ▒▒▒▒▒▒▒▒       ▒▒▒  ▒▒▒▒  ▒▒  ▒  ▒  ▒▒  ▒▒▒▒▒▒▒▒        ▒▒▒▒    ▒▒▒▒ ▒▒ ▒
▓  ▓▓▓▓  ▓▓  ▓▓▓  ▓▓▓  ▓▓▓▓  ▓▓  ▓▓    ██  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓  ▓ ▒
██      ███  ████  ███      ███  ███   ███      ███  ████  █████  ████▓▓▓ ▒
 __    __  ________  ________  __         ______   ____________________ __ _`, YELLOW)
	} else {
		fmt.Printf(`%s██      ███       ███  ████  ██   ███  ███      ███  ████  ██  ████  █
▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓    ▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓  ▓▓  ▓▓
▒  ▒▒▒▒▒▒▒▒       ▒▒▒  ▒▒▒▒  ▒▒  ▒  ▒  ▒▒  ▒▒▒▒▒▒▒▒        ▒▒▒▒    ▒▒▒
▓  ▓▓▓▓  ▓▓  ▓▓▓  ▓▓▓  ▓▓▓▓  ▓▓  ▓▓    ██  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓
██      ███  ████  ███      ███  ███   ███      ███  ████  █████  ████
 __    __  ________  ________  __         ______   __________________`, YELLOW)
	}
}

// ShowBanner prints ASCII art and system info
func showBanner() {
	clearScreen()
	ctx, cancel := context.WithCancel(context.Background())
	go asyncSpinner(ctx, "RAM...")
	usedRam := getRAMUsagePercent()
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	go asyncSpinner(ctx, "Uptime...")
	uptime := getUptime()
	cancel()

	ctx, cancel = context.WithCancel(context.Background())
	go asyncSpinner(ctx, "Time...")
	now := time.Now()
	cancel()

	clearScreen()
	line()
	crunchytext()
	fmt.Printf(`%s
█  ████  ██        ██        ██  █████████      ██ By: Knuspii, (M)
▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓ Version  : %s
▒  ▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒▒▒▒▒      ▒▒ Uptime   : %s
▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓ Used-RAM : %s
██      ██████  █████        ██        ███      ██ Time: %s
`, YELLOW, CU_VERSION, uptime, usedRam, now.Format("15:04:05"))
	line()
	fmt.Printf("Tools:\n")
	fmt.Printf("  [1]  - %sSystem monitor%s\n", YELLOW, RC)
	fmt.Printf("  [2]  - %sDoes a system cleanup%s\n", YELLOW, RC)
	fmt.Printf("  [3]  - %sClipboard logger%s\n", YELLOW, RC)
	fmt.Printf("  [4]  - %sTimer and stopwatch%s\n", YELLOW, RC)
	fmt.Printf("  [5]  - %sShutdown timer%s\n", YELLOW, RC)
	fmt.Printf("  [6]  - %sShow weather infos%s                        [U]  - %sUpdate%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [7]  - %sShow infos about domain%s                   [I]  - %sInfos%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [8]  - %sRestart display-manager%s                   [R]  - %sRestart%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [9]  - %sReboot to BIOS%s                            [Q]  - %sQuit%s\n", YELLOW, RC, RED, RC)
	line()
	fmt.Printf("Press key to launch a tool (1-9)\n")
}

//
// ========================== MAIN LOOP ==========================
//

func main() {
	flag.Parse()

	if *Flagskip {
		CMDWAIT = 0
	}

	if *Flagversion {
		fmt.Printf("CrunchyUtils %s\n", CU_VERSION)
		os.Exit(0)
	}

	startup()

	if err := keyboard.Open(); err != nil {
		panic(err)
	}
	defer keyboard.Close()

	for consoleRunning {
		showBanner()

		char, _, err := keyboard.GetSingleKey()
		if err != nil {
			fmt.Println("Error reading key:", err)
			pause()
			continue
		}

		cmdline()
		switch char {
		case '1':
			fmt.Printf("Starting CrunchySystemMonitor...\n")
			CrunchySystemMonitor()
			pause()
		case '2':
			printCommandTitle("Cleanup")
			if yesNo("Are you sure you want to do a cleanup?") {
				cleanSystemFull()
			}
		case '3':
			clipboardLogger()
			pause()
		case '4':
			timerMenu()
		case '5':
			powerMenu()
		case '6':
			weather()
		case '7':
			infograb()
		case '8':
			printCommandTitle("Restart Display-Manager")
			if yesNo("Sure you want to restart your display-manager?") {
				restartDisplay()
			}
		case '9':
			printCommandTitle("Reboot to BIOS")
			if yesNo("Sure you want to reboot to BIOS?") {
				rebootBIOS()
			}
		case 'u', 'U':
			printCommandTitle("Update CrunchyUtils")
			printInfo("CURRENTLY UNAVAILABLE")
			pause()
		case 'i', 'I':
			printInfo("CrunchyUtils " + CU_VERSION)
			pause()
		case 'r', 'R':
			clearScreen()
			startup()
		case 'q', 'Q', '0':
			consoleRunning = false
		default:
			fmt.Println("Invalid key")
			time.Sleep(2 * time.Second)
		}
	}
	printInfo("CrunchyUtils EXITED")
}
