// ##################################
// CrunchyUtils
// Made by: Knuspii, (M)
// Code is so bad it might explode
//
// cu_main.go: Menu, UI, starting point
// ##################################

package main

import (
	"bufio"   // For reading user input
	"context" // For controlling goroutines (e.g., stopping the spinner)
	"flag"
	"fmt"     // Formatted input/output
	"os"      // General OS interactions (exit, files, etc.)
	"os/exec" // Executing external commands
	"os/user" // Getting information about the current user
	"runtime" // Info about the OS / architecture
	"strings" // String manipulation (Trim, Split, Join, etc.)
	"time"    // Time-related functions (sleep, timestamp, timeout)

	"github.com/eiannone/keyboard"
	"github.com/gen2brain/beeep"
)

const (
	CU_VERSION = "0.16"
	COLS       = 70
	LINES      = 27
	PROMPT     = (YELLOW + " >>:" + RC) // Prompt string displayed to the user
	RED        = "\033[31m"
	YELLOW     = "\033[33m"
	GREEN      = "\033[32m"
	BLUE       = "\033[34m"
	CYAN       = "\033[36m"
	RC         = "\033[0m" // Reset ANSI color
)

var (
	consoleRunning = true
	goos           = runtime.GOOS
	reader         = bufio.NewReader(os.Stdin)
	SPINNERFRAMES  = []rune{'|', '/', '-', '\\'}
	CMDWAIT        = 1 * time.Second // Wait time running a command
	// Flags
	Flagversion = flag.Bool("version", false, "show version")
	Flagskip    = flag.Bool("skip", false, "skip all delays")
	Flagnoadmin = flag.Bool("no-admin", false, "skip admin request")
)

// ============================================================ CONSOLE / STARTUP ============================================================
// getAdmin checks if the program has administrative/root privileges.
// If not, it tries to restart itself with elevated rights (Windows) or sudo (Unix-like systems).
func getAdmin() {
	switch goos {
	// WINDOWS
	case "windows":
		// Try running a command that requires admin privileges
		cmd := exec.Command("net", "session")
		if err := cmd.Run(); err != nil {
			// If it fails, try to restart the program as admin
			printInfo("Restarting as admin...\n")

			elevate := exec.Command("powershell", "-Command", "Start-Process", os.Args[0], "-Verb", "RunAs")

			// Connect the elevated process output to the current console
			elevate.Stdout = os.Stdout
			elevate.Stderr = os.Stderr

			if err := elevate.Run(); err != nil {
				// Failed to elevate privileges
				printError(fmt.Sprintf("Failed to restart as admin: %v", err))
				fmt.Println("CrunchyUtils might not work correctly without admin rights")
				pause()
			} else {
				// Successfully elevated, exit current process
				os.Exit(0)
			}
		}
	// LINUX
	default:
		if os.Geteuid() != 0 { // Check if current user is not root
			printInfo("Requesting root privileges...\n")

			// Get absolute path to the running executable
			scriptPath, err := exec.LookPath(os.Args[0])
			if err != nil {
				printError(fmt.Sprintf("Could not find executable: %v", err))
				return
			}

			// Prepare command with all original arguments
			args := append([]string{scriptPath}, os.Args[1:]...)
			cmd := exec.Command("sudo", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin // Needed to read sudo password

			if err := cmd.Run(); err != nil {
				// User canceled or sudo failed
				if strings.Contains(err.Error(), "interrupt") {
					fmt.Println("\nCancelled by user")
					os.Exit(0)
				}
				printError(fmt.Sprintf("Failed to restart with sudo: %v", err))
				pause()
			} else {
				// Successfully elevated, exit current process
				os.Exit(0)
			}
		}
	}
}

// startup prints startup info
func startup() {
	clearScreen()
	printInfo("CrunchyUtils loading...")
	fmt.Printf("Version: %s\n", CU_VERSION)
	fmt.Printf("*******************\n")
	fmt.Printf("*  CrunchyUtils   *\n")
	fmt.Printf("*  c2025 Knuspii  *\n")
	fmt.Printf("*******************\n")
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
	printInfo("Initializing...")
	time.Sleep(CMDWAIT)
	time.Sleep(CMDWAIT)
	initApp()
	printSuccess("LOADING SUCCESSFUL!")
	time.Sleep(CMDWAIT)
	time.Sleep(CMDWAIT)
	go beeep.Beep(600, 150)
}

// initApp runs initialization logic and script
func initApp() {
	// Current user
	usr, err := user.Current()
	if err != nil {
		printError("Username: unknown")
	} else {
		printInfo(fmt.Sprintf("Username: %s", usr.Username))
	}

	// Set terminal title
	fmt.Printf("\033]0;CrunchyUtils\007")

	// Try to resize terminal
	switch goos {
	case "windows":
		// CMD title
		_, _ = runCommand([]string{"cmd", "/C", "title CrunchyUtils"})

		// PowerShell resize
		psCmd := fmt.Sprintf(
			`$Host.UI.RawUI.WindowSize = New-Object System.Management.Automation.Host.Size(%d,%d); $Host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size(%d,300)`,
			COLS, LINES, COLS,
		)
		_, _ = runCommand([]string{"powershell", "-Command", psCmd})

	default:
		// ANSI resize (best effort)
		fmt.Printf("\033[8;%d;%dt", LINES, COLS)
	}

	// ---- VERIFY SIZE ----
	var cols, lines int
	var sizeErr error

	if goos == "windows" {
		out, err := runCommand([]string{"cmd", "/C", "mode con"})
		if err != nil {
			sizeErr = err
		} else {
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "Columns:") {
					fmt.Sscanf(line, "    Columns: %d", &cols)
				}
				if strings.Contains(line, "Lines:") {
					fmt.Sscanf(line, "    Lines: %d", &lines)
				}
			}
		}
	} else {
		out, err := runCommand([]string{"sh", "-c", "stty size < /dev/tty"})
		if err != nil {
			sizeErr = err
		} else {
			fmt.Sscanf(out, "%d %d", &lines, &cols)
		}
	}

	if sizeErr != nil || cols == 0 || lines == 0 {
		printError("Could not detect terminal size")
		return
	}

	if cols != COLS || lines != LINES {
		printError(fmt.Sprintf("Terminal size mismatch got: %dx%d expected: %dx%d", cols, lines, COLS, LINES))
	} else {
		printInfo(fmt.Sprintf("Terminal size OK: %dx%d", cols, lines))
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
	go asyncSpinner(ctx, "CPU tasks...")
	topCPU := getTopCPUProcesses()
	cancel()

	clearScreen()
	line()
	fmt.Printf(`%s██      ███       ███  ████  ██   ███  ███      ███  ████  ██  ████  █
▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓    ▓▓  ▓▓  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓  ▓▓  ▓▓
▒  ▒▒▒▒▒▒▒▒       ▒▒▒  ▒▒▒▒  ▒▒  ▒  ▒  ▒▒  ▒▒▒▒▒▒▒▒        ▒▒▒▒    ▒▒▒
▓  ▓▓▓▓  ▓▓  ▓▓▓  ▓▓▓  ▓▓▓▓  ▓▓  ▓▓    ██  ▓▓▓▓  ▓▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓
██      ███  ████  ███      ███  ███   ███      ███  ████  █████  ████
 __    __  ________  ________  __         ______   __________________
█  ████  ██        ██        ██  █████████      ██ By: Knuspii, (M)
▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓▓▓ Version  : %s
▒  ▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒▒▒▒  ▒▒▒▒▒  ▒▒▒▒▒▒▒▒▒      ▒▒ Uptime   : %s
▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓ Used-RAM : %s
██      ██████  █████        ██        ███      ██ Top CPU Tasks:
`, YELLOW, CU_VERSION, uptime, usedRam)
	fmt.Printf("%s#════════════════════════════════════════════════╗ ├%s%s\n", YELLOW, topCPU[0], RC)
	fmt.Printf("Tools:                                           %s║ ├%s%s\n", YELLOW, topCPU[1], RC)
	fmt.Printf("  [1]  - %sSystem monitor%s                          %s║ └%s%s\n", YELLOW, RC, YELLOW, topCPU[2], RC)
	fmt.Printf("  [2]  - %sDoes a system cleanup%s                   %s╚═══════════════════#%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [3]  - %sClipboard logger%s\n", YELLOW, RC)
	fmt.Printf("  [4]  - %sTimer and stopwatch%s                       [I]  - %sInfos%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [5]  - %sShutdown timer%s                            [R]  - %sRestart%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [6]  - %sShow weather infos%s                        [Q]  - %sQuit%s\n", YELLOW, RC, RED, RC)
	fmt.Printf("  [7]  - %sShow infos about domain%s\n", YELLOW, RC)
	fmt.Printf("  [8]  - %sRestart display-manager%s\n", YELLOW, RC)
	fmt.Printf("  [9]  - %sReboot to BIOS%s\n", YELLOW, RC)
	line()
	fmt.Printf("Press key to launch a tool (1-9)\n")
}

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
			continue
		}

		cmdline()
		switch char {
		case '1':
			keyboard.Close()
			fmt.Printf("Starting CrunchySystemMonitor...\n")
			CrunchySystemMonitor()
			pause()
		case '2':
			keyboard.Close()
			printCommandTitle("Cleanup")
			if yesNo("Are you sure you want to do a full cleanup?") {
				cleanSystemFull()
			}
		case '3':
			keyboard.Close()
			clipboardLogger()
			pause()
		case '4':
			// Timer/Stopwatch Menu
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
				case '0':
					goto endTimerMenu
				case '1':
					keyboard.Close()
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
					keyboard.Close()
					stopwatch()
				default:
					fmt.Printf("\nInvalid Option\n")
					time.Sleep(2 * time.Second)
				}
				pause()
			}
		endTimerMenu:
			fmt.Printf("Closed Timer Menu")
		case '5':
			// PowerTimer Menu
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
				case '0':
					goto endPowerMenu
				case '1':
					keyboard.Close()
					if goos == "windows" {
						toption = "Wshutdown"
					} else {
						toption = "Lshutdown"
					}
					powerTimer("shutdown/reboot", toption)
				case '2':
					keyboard.Close()
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
		endPowerMenu:
			fmt.Printf("Closed Power Menu")
		case '6':
			keyboard.Close()
			weather()
		case '7':
			keyboard.Close()
			infograb()
		case '8':
			keyboard.Close()
			printCommandTitle("Restart Display-Manager")
			if yesNo("Sure you want to restart your display-manager?") {
				restartDisplay()
			}
		case '9':
			keyboard.Close()
			printCommandTitle("Reboot to BIOS")
			if yesNo("Sure you want to reboot to BIOS?") {
				rebootBIOS()
			}
		case 'i', 'I':
			keyboard.Close()
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
	keyboard.Close()
	printInfo("EXITED")
}
