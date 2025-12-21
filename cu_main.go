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
	"fmt"     // Formatted input/output
	"os"      // General OS interactions (exit, files, etc.)
	"os/exec" // Executing external commands
	"os/user" // Getting information about the current user

	// Handling file paths in a cross-platform way
	"runtime" // Info about the OS / architecture
	"strings" // String manipulation (Trim, Split, Join, etc.)
	"time"    // Time-related functions (sleep, timestamp, timeout)

	"github.com/eiannone/keyboard"
)

const (
	CU_VERSION = "0.15"
	COLS       = 70
	LINES      = 25
	CMDWAIT    = 0 * time.Second        // Wait time running a command
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
	//SPINNERFRAMES  = []rune{'⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷'}
	SPINNERFRAMES = []rune{'|', '/', '-', '\\'}
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
	time.Sleep(CMDWAIT)
	switch goos {
	case "windows":
		fmt.Printf("%sWindows Detected%s\n", CYAN, RC)
	case "linux":
		fmt.Printf("%sLinux Detected%s\n", YELLOW, RC)
	default:
		printError(goos + " not supported!")
		pause()
		os.Exit(1)
	}
	now := time.Now()
	fmt.Printf("Time: %s //// Date: %s\n\n", now.Format("15:04:05"), now.Format("02.01.2006"))
	time.Sleep(CMDWAIT)
	getAdmin()
	time.Sleep(CMDWAIT)
	initApp()
	fmt.Printf("%s\n*************************************", GREEN)
	printSuccess("LOADING SUCCESSFUL!")
	time.Sleep(CMDWAIT)
	time.Sleep(CMDWAIT)
}

// initApp runs initialization logic and script
func initApp() {
	printInfo("Initializing...")

	// Get current username
	usr, err := user.Current()
	if err != nil {
		fmt.Printf("Username: unknown\n")
	} else {
		fmt.Printf("Username: %s\n", usr.Username)
	}

	// Set terminal title (works in most terminals)
	fmt.Printf("\033]0;CrunchyUtils\007")

	switch goos {
	case "windows":
		// Set Windows CMD title
		_, err := runCommand([]string{"cmd", "/C", "title CrunchyUtils"})
		if err != nil {
			printError("Failed to set CMD title: " + err.Error())
		}

		// Set PowerShell window size and buffer
		psCmd := fmt.Sprintf(
			`$Host.UI.RawUI.WindowSize = New-Object System.Management.Automation.Host.Size(%d, %d); $Host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size(%d, 300)`,
			COLS, LINES, COLS,
		)
		_, err = runCommand([]string{"powershell", "-Command", psCmd})
		if err != nil {
			printError("Failed to set window size: " + err.Error())
		}

	default:
		// Set Unix terminal size using ANSI escape codes
		fmt.Printf("\033[8;%d;%dt", LINES, COLS)
	}
}

// ShowBanner prints ASCII art and system info
func showBanner() {
	clearScreen()
	ctx, cancel := context.WithCancel(context.Background())
	go asyncSpinner(ctx, "RAM...")
	freeRam := getFreeRAMPercent()
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
▓  ▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓  ▓▓▓▓▓  ▓▓▓▓▓▓▓▓▓▓▓▓▓▓  ▓ Free-RAM : %s
██      ██████  █████        ██        ███      ██ Top CPU Tasks:
`, YELLOW, CU_VERSION, uptime, freeRam)
	fmt.Printf("%s#════════════════════════════════════════════════╗ ├%s%s\n", YELLOW, topCPU[0], RC)
	fmt.Printf("Tools:                                           %s║ ├%s%s\n", YELLOW, topCPU[1], RC)
	fmt.Printf("  [1]  - %sSystem monitor%s                          %s║ └%s%s\n", YELLOW, RC, YELLOW, topCPU[2], RC)
	fmt.Printf("  [2]  - %sDoes a system cleanup%s                   %s╚═══════════════════#%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [3]  - %sClipboard logger%s\n", YELLOW, RC)
	fmt.Printf("  [4]  - %sTimer and stopwatch%s                       [I]  - %sInfos%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [5]  - %sShutdown timer%s                            [R]  - %sRefresh%s\n", YELLOW, RC, YELLOW, RC)
	fmt.Printf("  [6]  - %sShow weather infos%s                        [E]  - %sExit%s\n", YELLOW, RC, RED, RC)
	fmt.Printf("  [7]  - %sShow infos about Domain%s\n", YELLOW, RC)
	line()
	fmt.Printf("Press key to launch a tool (1-7)\n")
}

func main() {
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

		switch char {
		case '1':
			CrunchySystemMonitor()
			pause()
		case '2':
			printCommandTitle("Cleanup")
			if yesNo("Are you sure you want to do a full cleanup?") {
				cleanSystemFull()
			}
			pause()
		case '3':
			clipboardLogger()
			pause()
		case '4':
			timerOrStopwatchMenu()
		case '5':
			powerTimerMenu()
		case '6':
			weather()
		case '7':
			infograb()
		case 'i':
			printInfo("CrunchyUtils Version: " + CU_VERSION)
			pause()
		case 'r':
			clearScreen()
			startup()
		case 'e':
			printInfo("EXITED")
			consoleRunning = false
		default:
			fmt.Println("Invalid key")
		}
	}
}
