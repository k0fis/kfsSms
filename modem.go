package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Modem struct {
	portName string
	baudRate int
	port     serial.Port
}

func NewModem(portName string, baudRate int) *Modem {
	return &Modem{portName: portName, baudRate: baudRate}
}

// DetectPort tries all available COM ports with AT command and returns the one that responds.
func DetectPort(baudRate int) (string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return "", fmt.Errorf("cannot list ports: %w", err)
	}
	if len(ports) == 0 {
		return "", fmt.Errorf("no serial ports found")
	}

	log.Printf("[INFO] scanning ports: %v", ports)

	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	for _, portName := range ports {
		port, err := serial.Open(portName, mode)
		if err != nil {
			log.Printf("[DEBUG] %s: cannot open: %v", portName, err)
			continue
		}
		port.SetReadTimeout(500 * time.Millisecond)

		// Send AT and look for OK
		port.Write([]byte("AT\r"))
		time.Sleep(600 * time.Millisecond)

		buf := make([]byte, 256)
		var resp strings.Builder
		for {
			n, _ := port.Read(buf)
			if n == 0 {
				break
			}
			resp.Write(buf[:n])
		}
		port.Close()

		if strings.Contains(resp.String(), "OK") {
			log.Printf("[INFO] modem detected on %s", portName)
			return portName, nil
		}
		log.Printf("[DEBUG] %s: no AT response", portName)
	}

	return "", fmt.Errorf("no modem found on any port (tried %d ports)", len(ports))
}

func (m *Modem) Open(pin string) error {
	mode := &serial.Mode{
		BaudRate: m.baudRate,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	port, err := serial.Open(m.portName, mode)
	if err != nil {
		return fmt.Errorf("cannot open port %s: %w", m.portName, err)
	}
	m.port = port
	m.port.SetReadTimeout(100 * time.Millisecond)

	// Basic init
	if _, err := m.send("AT", 2*time.Second); err != nil {
		return fmt.Errorf("modem handshake failed: %w", err)
	}
	if _, err := m.send("ATE0", 2*time.Second); err != nil {
		return fmt.Errorf("echo off failed: %w", err)
	}
	if _, err := m.send("AT+CMGF=1", 2*time.Second); err != nil {
		return fmt.Errorf("text mode failed: %w", err)
	}

	if err := m.ensureSimReady(pin); err != nil {
		return err
	}

	return nil
}

func (m *Modem) Close() {
	if m.port != nil {
		m.port.Close()
	}
}

func (m *Modem) SendSms(number, message string) error {
	// AT+CMGS="number" → wait for prompt '>'
	resp, err := m.sendExpectPrompt(fmt.Sprintf(`AT+CMGS="%s"`, number), '>', 2*time.Second)
	if err != nil {
		return fmt.Errorf("CMGS prompt failed: %w", err)
	}
	if !strings.Contains(resp, ">") {
		return fmt.Errorf("no prompt received: %s", resp)
	}

	// Send message body + CTRL-Z
	m.port.Write([]byte(message))
	m.port.Write([]byte{26}) // CTRL-Z

	// Wait for OK/+CMGS
	result, err := m.readUntil("OK", 10*time.Second)
	if err != nil {
		return fmt.Errorf("send timeout: %w", err)
	}
	if !strings.Contains(result, "+CMGS") {
		return fmt.Errorf("SMS not confirmed: %s", result)
	}

	log.Printf("[INFO] SMS sent number=%s", number)
	return nil
}

func (m *Modem) ReadAll() ([]SmsMessage, error) {
	resp, err := m.send("AT+CMGL=\"ALL\"", 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("CMGL failed: %w", err)
	}
	return ParseCMGL(resp), nil
}

func (m *Modem) Delete(index int) error {
	_, err := m.send(fmt.Sprintf("AT+CMGD=%d", index), 2*time.Second)
	return err
}

// --- internal ---

func (m *Modem) ensureSimReady(pin string) error {
	log.Printf("[INFO] checking SIM PIN")

	resp, err := m.send("AT+CPIN?", 5*time.Second)
	if err != nil {
		return fmt.Errorf("CPIN query failed: %w", err)
	}

	if strings.Contains(resp, "READY") {
		log.Printf("[INFO] SIM already ready")
	} else if strings.Contains(resp, "SIM PIN") {
		if pin == "" {
			return fmt.Errorf("SIM requires PIN but none provided")
		}
		log.Printf("[INFO] sending SIM PIN")
		if _, err := m.send(fmt.Sprintf(`AT+CPIN="%s"`, pin), 5*time.Second); err != nil {
			return fmt.Errorf("PIN send failed: %w", err)
		}
		time.Sleep(5 * time.Second)

		verify, err := m.send("AT+CPIN?", 5*time.Second)
		if err != nil || !strings.Contains(verify, "READY") {
			return fmt.Errorf("PIN not accepted: %s", verify)
		}
		log.Printf("[INFO] SIM unlocked")
	} else if strings.Contains(resp, "SIM PUK") {
		return fmt.Errorf("SIM blocked (PUK required)")
	} else {
		return fmt.Errorf("unknown CPIN response: %s", resp)
	}

	// Wait for network registration
	log.Printf("[INFO] waiting for network registration")
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		reg, err := m.send("AT+CEREG?", 5*time.Second)
		if err == nil && (strings.Contains(reg, ",1") || strings.Contains(reg, ",5")) {
			log.Printf("[INFO] network registered")
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("network registration timeout")
}

func (m *Modem) send(cmd string, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	return m.readUntil("OK", timeout)
}

func (m *Modem) sendExpectPrompt(cmd string, prompt byte, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	return m.readUntil(string(prompt), timeout)
}

func (m *Modem) writeLine(cmd string) {
	m.port.Write([]byte(cmd + "\r"))
}

func (m *Modem) readUntil(expected string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var sb strings.Builder
	buf := make([]byte, 256)

	for time.Now().Before(deadline) {
		n, _ := m.port.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			if strings.Contains(sb.String(), expected) {
				return sb.String(), nil
			}
		}
		if n == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return sb.String(), fmt.Errorf("timeout waiting for %q, got: %s", expected, sb.String())
}
