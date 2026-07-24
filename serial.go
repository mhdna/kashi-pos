package main

import (
	"os/exec"
	"runtime"
	"strings"
	"time"

	"go.bug.st/serial"
)

func (a *App) ListSerialPorts() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	if ports == nil {
		return []string{}, nil
	}
	return ports, nil
}

func (a *App) ListPrinters() ([]string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", "Get-Printer | Select-Object -ExpandProperty Name")
	} else {
		cmd = exec.Command("lpstat", "-e")
	}
	out, err := cmd.Output()
	if err != nil {
		return []string{}, nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			result = append(result, l)
		}
	}
	if result == nil {
		return []string{}, nil
	}
	return result, nil
}

func (a *App) WriteToScreen(port string, message string) error {
	mode := &serial.Mode{BaudRate: 2400}
	p, err := serial.Open(port, mode)
	if err != nil {
		return err
	}
	defer p.Close()

	_, _ = p.Write([]byte{0x1B, 0x40})
	time.Sleep(50 * time.Millisecond)
	_, err = p.Write([]byte(message))
	return err
}

func (a *App) ClearScreen(port string) error {
	mode := &serial.Mode{BaudRate: 2400}
	p, err := serial.Open(port, mode)
	if err != nil {
		return err
	}
	defer p.Close()

	_, _ = p.Write([]byte{0x1B, 0x40})
	return nil
}

func (a *App) WriteAndClearScreen(port string, total string) error {
	if err := a.WriteToScreen(port, total); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return a.ClearScreen(port)
}
