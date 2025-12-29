package main

import (
	"fmt"
	"log"
	"syscall"
	"time"
	"unsafe"
)

// Windows API Constants
const (
	KEY_READ                  = 0x20019
	HKEY_CURRENT_USER         = 0x80000001
	TH32CS_SNAPPROCESS        = 0x00000002
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
	PROCESS_TERMINATE         = 0x0001
	ERROR_ALREADY_EXISTS      = 183
)

var (
	modadvapi32 = syscall.NewLazyDLL("advapi32.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	moduser32   = syscall.NewLazyDLL("user32.dll")

	procRegOpenKeyExW    = modadvapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueExW = modadvapi32.NewProc("RegQueryValueExW")
	procRegCloseKey      = modadvapi32.NewProc("RegCloseKey")

	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = modkernel32.NewProc("Process32FirstW")
	procProcess32NextW           = modkernel32.NewProc("Process32NextW")
	procOpenProcess              = modkernel32.NewProc("OpenProcess")
	procGetProcessIoCounters     = modkernel32.NewProc("GetProcessIoCounters")
	procTerminateProcess         = modkernel32.NewProc("TerminateProcess")
	procCloseHandle              = modkernel32.NewProc("CloseHandle")
	procCreateMutexW             = modkernel32.NewProc("CreateMutexW")

	procMessageBoxW = moduser32.NewProc("MessageBoxW")
)

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type PROCESSENTRY32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

func createMutex(name string) (uintptr, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	ret, _, err := procCreateMutexW.Call(
		0,
		0, // FALSE for initial owner
		uintptr(unsafe.Pointer(namePtr)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("CreateMutex failed: %v", err)
	}

	if err != nil {
		if errno, ok := err.(syscall.Errno); ok {
			if errno == ERROR_ALREADY_EXISTS {
				return ret, fmt.Errorf("another instance is already running")
			}
		}
	}
	return ret, nil
}

func showErrorBox(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0x00000030, // MB_OK | MB_ICONWARNING
	)
}

func showInfoBox(title, message string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		0x00000040, // MB_OK | MB_ICONINFORMATION
	)
}

func main() {
	fmt.Println("IDM Auto Closer Started...")

	// Use a unique name without Global\ prefix to avoid permission issues,
	// but specific enough to be unique system-wide if possible (in Local namespace).
	// If running in different folders, we still want single instance.
	const mutexName = "IDM_Auto_Closer_Unique_Mutex_v1"
	handle, err := createMutex(mutexName)
	if err != nil {
		showErrorBox("IDM Auto Closer", "程序已在运行中！\nProgram is already running.")
		return
	}
	// Ensure handle is closed when main exits (though process exit does this too)
	defer procCloseHandle.Call(handle)

	showInfoBox("IDM Auto Closer", "欢迎使用 IDM 自动关闭工具！\n\n本程序将在后台运行，监测 IDM 下载任务。\n当所有下载完成后，会自动关闭 IDM 进程。\n\nWelcome to IDM Auto Closer!\n\nThis program runs in the background monitoring IDM.\nIt will automatically close IDM when downloads finish.")

	fmt.Println("Monitoring IDM activity...")

	idmPath, err := getIDMPath()
	if err != nil {
		log.Printf("Warning: Could not find IDM path in registry: %v\n", err)
		log.Println("Will attempt to find process by name 'IDMan.exe' anyway.")
	} else {
		fmt.Printf("IDM Path found: %s\n", idmPath)
	}

	exeName := "IDMan.exe"

	// State machine
	// 0: Waiting for IDM to start
	// 1: Waiting for download activity (High traffic)
	// 2: Monitoring download (Waiting for finish)
	state := 0

	// Configuration
	const (
		downloadThresholdSpeed = 5 * 1024         // 5 KB/s - Threshold to detect download start
		idleThresholdSpeed     = 1 * 1024         // 1 KB/s - Threshold to detect idle state
		checkInterval          = 1 * time.Second  // Check interval
		finishDelay            = 30 * time.Second // Delay to confirm download finished (increased to 30s)
		consecutiveIdleCount   = 5                // Number of consecutive idle checks to confirm finish
	)

	var lastReadCount uint64
	var lastWriteCount uint64 // Monitor write activity as well
	var lastCheckTime time.Time
	var lowActivityStartTime time.Time
	var idleCheckCount int // Consecutive idle counter
	var pid uint32
	var hProcess syscall.Handle

	for {
		time.Sleep(checkInterval)

		// Check if process exists
		currentPid, err := findProcessID(exeName)
		if err != nil {
			// Process not running
			if state != 0 {
				fmt.Println("IDM closed. Resetting state.")
				if hProcess != 0 {
					syscall.CloseHandle(hProcess)
					hProcess = 0
				}
				state = 0
			}
			continue
		}

		// Process found
		if state == 0 {
			fmt.Printf("IDM detected (PID: %d). Waiting for download activity...\n", currentPid)
			pid = currentPid
			hProcess, err = openProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ|PROCESS_TERMINATE, false, pid)
			if err != nil {
				log.Printf("Error opening process: %v\n", err)
				continue
			}

			// Initialize counters
			io, err := getProcessIoCounters(hProcess)
			if err == nil {
				lastReadCount = io.ReadTransferCount
				lastWriteCount = io.WriteTransferCount
				lastCheckTime = time.Now()
			}
			idleCheckCount = 0
			state = 1
			continue
		}

		// If PID changed (IDM restarted), reset
		if currentPid != pid {
			fmt.Println("IDM PID changed. Resetting.")
			syscall.CloseHandle(hProcess)
			state = 0
			continue
		}

		// Get IO Counters
		io, err := getProcessIoCounters(hProcess)
		if err != nil {
			log.Printf("Error getting IO counters: %v. Resetting.\n", err)
			syscall.CloseHandle(hProcess)
			state = 0
			continue
		}

		now := time.Now()
		duration := now.Sub(lastCheckTime).Seconds()
		if duration == 0 {
			duration = 1
		}

		// Calculate speed (Bytes per second) - consider both read and write activity
		bytesRead := io.ReadTransferCount - lastReadCount
		bytesWritten := io.WriteTransferCount - lastWriteCount
		totalBytes := bytesRead + bytesWritten
		readSpeed := float64(bytesRead) / duration
		totalSpeed := float64(totalBytes) / duration

		lastReadCount = io.ReadTransferCount
		lastWriteCount = io.WriteTransferCount
		lastCheckTime = now

		// State Logic
		switch state {
		case 1: // Waiting for download start
			if readSpeed > downloadThresholdSpeed {
				fmt.Printf("Download activity detected (Speed: %.2f KB/s). Monitoring...\n", readSpeed/1024)
				state = 2
				lowActivityStartTime = time.Time{}
				idleCheckCount = 0
			}
		case 2: // Monitoring download
			// Use total IO activity (read + write) to determine idle state
			// This helps accurately detect if download has really stopped
			if totalSpeed < idleThresholdSpeed {
				idleCheckCount++
				if lowActivityStartTime.IsZero() {
					lowActivityStartTime = now
					fmt.Printf("Low activity detected (Speed: %.2f KB/s). Waiting for confirmation...\n", totalSpeed/1024)
				}
				// Both conditions must be met: consecutive idle checks AND idle duration
				idleDuration := now.Sub(lowActivityStartTime)
				if idleCheckCount >= consecutiveIdleCount && idleDuration >= finishDelay {
					fmt.Printf("Download finished confirmed (idle for %.1f seconds, %d consecutive checks). Closing IDM...\n",
						idleDuration.Seconds(), idleCheckCount)

					// Kill Process
					err := terminateProcess(hProcess, 0)
					if err != nil {
						log.Printf("Failed to terminate IDM: %v\n", err)
					} else {
						fmt.Println("IDM Terminated.")
					}

					// Reset
					syscall.CloseHandle(hProcess)
					hProcess = 0
					state = 0
					idleCheckCount = 0
				}
			} else {
				// Still downloading - activity detected, reset idle counter
				if idleCheckCount > 0 || !lowActivityStartTime.IsZero() {
					fmt.Printf("Download activity resumed (Speed: %.2f KB/s). Resetting idle counter.\n", totalSpeed/1024)
					lowActivityStartTime = time.Time{}
					idleCheckCount = 0
				}
			}
		}
	}
}

// Helper Functions

func getIDMPath() (string, error) {
	var hKey uintptr
	ret, _, _ := procRegOpenKeyExW.Call(
		uintptr(HKEY_CURRENT_USER),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(`Software\DownloadManager`))),
		0,
		uintptr(KEY_READ),
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegOpenKeyExW failed with error code %d", ret)
	}
	defer procRegCloseKey.Call(hKey)

	var bufLen uint32 = 1024
	buf := make([]uint16, bufLen)
	ret, _, _ = procRegQueryValueExW.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("ExePath"))),
		0,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegQueryValueExW failed with error code %d", ret)
	}

	return syscall.UTF16ToString(buf), nil
}

func findProcessID(exeName string) (uint32, error) {
	snapshot, _, err := procCreateToolhelp32Snapshot.Call(uintptr(TH32CS_SNAPPROCESS), 0)
	if snapshot == uintptr(syscall.InvalidHandle) {
		return 0, err
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	var pe32 PROCESSENTRY32W
	pe32.Size = uint32(unsafe.Sizeof(pe32))

	r1, _, err := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
	if r1 == 0 {
		return 0, err
	}

	for {
		name := syscall.UTF16ToString(pe32.ExeFile[:])
		if name == exeName {
			return pe32.ProcessID, nil
		}

		r1, _, err = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&pe32)))
		if r1 == 0 {
			break
		}
	}

	return 0, fmt.Errorf("process not found")
}

func openProcess(desiredAccess uint32, inheritHandle bool, processId uint32) (syscall.Handle, error) {
	inherit := 0
	if inheritHandle {
		inherit = 1
	}
	r1, _, err := procOpenProcess.Call(uintptr(desiredAccess), uintptr(inherit), uintptr(processId))
	if r1 == 0 {
		return 0, err
	}
	return syscall.Handle(r1), nil
}

func getProcessIoCounters(hProcess syscall.Handle) (IO_COUNTERS, error) {
	var io IO_COUNTERS
	r1, _, err := procGetProcessIoCounters.Call(uintptr(hProcess), uintptr(unsafe.Pointer(&io)))
	if r1 == 0 {
		return io, err
	}
	return io, nil
}

func terminateProcess(hProcess syscall.Handle, exitCode uint32) error {
	r1, _, err := procTerminateProcess.Call(uintptr(hProcess), uintptr(exitCode))
	if r1 == 0 {
		return err
	}
	return nil
}
