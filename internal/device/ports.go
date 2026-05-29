package device

import (
	"fmt"
	"strings"

	"go.bug.st/serial/enumerator"
)

const SamsungVID = "04E8"

type Port struct {
	Name      string
	VID       string
	PID       string
	Product   string
	IsSamsung bool
}

func ListPorts() ([]Port, error) {
	details, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("enumerate serial ports: %w", err)
	}
	ports := make([]Port, 0, len(details))
	for _, d := range details {
		p := Port{Name: d.Name, Product: d.Product}
		if d.IsUSB {
			p.VID = strings.ToUpper(d.VID)
			p.PID = strings.ToUpper(d.PID)
			p.IsSamsung = p.VID == SamsungVID
		}
		ports = append(ports, p)
	}
	return ports, nil
}

func DetectSamsungPort() (string, error) {
	ports, err := ListPorts()
	if err != nil {
		return "", err
	}
	for _, p := range ports {
		if p.IsSamsung {
			return p.Name, nil
		}
	}
	return "", fmt.Errorf("no Samsung serial port found (USB VID %s); connect over USB with USB debugging enabled, or pass --port", SamsungVID)
}
