package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	netWatcherMutex     sync.Mutex
	isNetWatcherRunning bool
	netWatcherExpiry    time.Time
)

func NetWatcher(macAddresses []string, cb func(success bool)) {
	netWatcherMutex.Lock()
	if isNetWatcherRunning {
		// Extend the expiry time by 5 seconds
		netWatcherExpiry = netWatcherExpiry.Add(5 * time.Second)
		lg.Debug("NetWatcher is already running, extended expiry time by 5 seconds")
		netWatcherMutex.Unlock()
		return
	}
	isNetWatcherRunning = true
	maxruntime := 10 * time.Second
	netWatcherExpiry = time.Now().Add(maxruntime)
	netWatcherMutex.Unlock()
	success := false

	defer func() {
		netWatcherMutex.Lock()
		isNetWatcherRunning = false
		netWatcherMutex.Unlock()
		cb(success)
	}()

	longInterval := 200 * time.Millisecond
	shortInterval := 20 * time.Millisecond

	matches, err := filepath.Glob("/sys/class/net/wl*")
	if err != nil {
		lg.Error("Could not glob wlans", "error", err.Error())
		return
	}

	allInterfacesDown := true
	for _, match := range matches {
		fullPath := fmt.Sprintf("%s/dormant", match)
		_, err := os.ReadFile(fullPath)
		if err == nil {
			allInterfacesDown = false
			break
		}
	}

	if allInterfacesDown {
		lg.Debug("All network interfaces are down, can't proceed with finding mac address")
		return
	}

	fakeAddr := "00:00:00:00:00:00"
	arpPath := "/proc/net/arp"
	var newStateBytes []byte
	for {
		if time.Now().After(netWatcherExpiry) {
			lg.Debug("Netwatcher: arp file still empty after timeout")
			return
		}
		newStateBytes = fileContentBytes(arpPath)
		if len(newStateBytes) > 79 {
			break
		}
		time.Sleep(longInterval)
	}

	var newState string
	for {
		if time.Now().After(netWatcherExpiry) {
			lg.Debug("Netwatcher: arp file not empty but wrong mac after timeout", "newstate", newState)
			return
		}

		newStateBytes = fileContentBytes(arpPath)
		newState = string(newStateBytes[79:])
		if !strings.Contains(newState, fakeAddr) {
			break
		}
		time.Sleep(shortInterval)
	}

	for _, macAddress := range macAddresses {
		lg.Debug("checking", "MAC", macAddress)
		if strings.Contains(newState, macAddress) {
			success = true
			return
		}
	}

	lg.Debug("Not connected to router with MAC")
}

func fileContentBytes(arpPath string) []byte {
	content, err := os.ReadFile(arpPath)
	if err != nil {
		lg.Error(err.Error())
	}
	return content
}

type ArpError struct {
	msg string
}

func NewArpError(msg string) *ArpError {
	return &ArpError{
		msg: msg,
	}
}

func (e *ArpError) Error() string { return e.msg }

func ExtractMac() (string, error) {
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip the header line
		if strings.Contains(line, "IP address") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			// ipAddress := fields[0]
			hwAddress := fields[3]
			return hwAddress, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", NewArpError("Could not find mac")
}
