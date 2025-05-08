package common

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

type FileSystemInterface interface {
	Stat(name string) (os.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	UserHomeDir() (string, error)
	Remove(name string) error
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type RealFileSystem struct{}

func (fs *RealFileSystem) Stat(name string) (os.FileInfo, error) { return os.Stat(name) }
func (fs *RealFileSystem) ReadFile(name string) ([]byte, error)  { return os.ReadFile(name) }
func (fs *RealFileSystem) UserHomeDir() (string, error)          { return os.UserHomeDir() }
func (fs *RealFileSystem) Remove(name string) error              { return os.Remove(name) }

func (fs *RealFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (fs *RealFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func KillProcessByPort(port int, commandName string) error {

	pids, err := findPIDsByPort(port, commandName)
	if err != nil {
		return fmt.Errorf("failed to find processes on port %d: %w", port, err)
	}

	if len(pids) == 0 {
		return nil
	}

	log.Printf("Terminating %d processes on port %d: %v", len(pids), port, pids)
	for _, pid := range pids {
		if pid <= 1000 && runtime.GOOS != "windows" {
			log.Printf("Skipping system process PID %d", pid)
			continue
		}
		if err := terminateProcess(pid); err != nil {
			return fmt.Errorf("failed to terminate process (PID %d) on port %d: %w", pid, port, err)
		}
	}

	return nil
}

func terminateProcess(pid int) error {
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		if isProcessFinished(err) {
			return nil
		}
		return fmt.Errorf("failed to find process %d: %w", pid, err)
	}

	err = proc.Terminate()
	if err != nil && !isProcessFinished(err) {
		log.Printf("Termination failed for PID %d: %v; falling back to Kill", pid, err)
		err = proc.Kill()
		if err != nil && !isProcessFinished(err) {
			return fmt.Errorf("failed to terminate process %d: %w", pid, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for process %d to terminate", pid)
		default:
			running, err := proc.IsRunning()
			if err != nil || !running {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func isProcessFinished(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "no such process") ||
		strings.Contains(errStr, "process already finished") ||
		strings.Contains(errStr, "The process cannot be accessed") ||
		strings.Contains(errStr, "process does not exist")
}

func findPIDsByPort(port int, commandName string) ([]int, error) {

	conns, err := net.Connections("tcp")
	if err != nil {
		return nil, fmt.Errorf("failed to list network connections: %w", err)
	}

	if commandName == "" {
		log.Printf("Warning: No commandName provided; including all processes on port %d", port)
	}

	pidSet := make(map[int]struct{})
	for _, conn := range conns {
		if conn.Status == "LISTEN" && conn.Laddr.Port == uint32(port) {
			pid := int(conn.Pid)
			if pid == 0 {
				continue
			}
			if commandName != "" {
				proc, err := process.NewProcess(int32(pid))
				if err != nil {
					log.Printf("Skipping PID %d: failed to get process: %v", pid, err)
					continue
				}
				name, err := proc.Name()
				if err != nil {
					log.Printf("Skipping PID %d: failed to get process name: %v", pid, err)
					continue
				}
				if !strings.Contains(strings.ToLower(name), strings.ToLower(commandName)) {
					continue
				}
			}
			pidSet[pid] = struct{}{}
		}
	}

	var pids []int
	for pid := range pidSet {
		pids = append(pids, pid)
	}
	return pids, nil
}

func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be between 1 and 65535)", port)
	}
	return nil
}
