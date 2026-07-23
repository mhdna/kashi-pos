package main

import (
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

func (a *App) WriteToScreen(port string, message string) error {
	mode := &serial.Mode{BaudRate: 2400}
	p, err := serial.Open(port, mode)
	if err != nil {
		return err
	}
	defer p.Close()

	p.Write([]byte{0x1B, 0x40})
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

	p.Write([]byte{0x1B, 0x40})
	return nil
}

func (a *App) WriteAndClearScreen(port string, total string) error {
	if err := a.WriteToScreen(port, total); err != nil {
		return err
	}
	time.Sleep(time.Second)
	return a.ClearScreen(port)
}
