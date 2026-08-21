package main

import (
	"fmt"
	"log/slog"
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

	slog.Info("scanning ports", "ports", ports)

	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	for _, portName := range ports {
		port, err := serial.Open(portName, mode)
		if err != nil {
			slog.Debug("port cannot open", "port", portName, "err", err)
			continue
		}
		port.SetReadTimeout(500 * time.Millisecond)

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
			slog.Info("modem detected", "port", portName)
			return portName, nil
		}
		slog.Debug("no AT response", "port", portName)
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

	// Flush any stale data in serial buffer from previous session
	m.drain()

	// Wake up modem — send bare CR to cancel any pending command, then AT
	m.port.Write([]byte("\r"))
	time.Sleep(200 * time.Millisecond)
	m.drain()

	if _, err := m.send("AT", 2*time.Second); err != nil {
		return fmt.Errorf("modem handshake failed: %w", err)
	}
	if _, err := m.send("ATE0", 2*time.Second); err != nil {
		return fmt.Errorf("echo off failed: %w", err)
	}

	// PIN must be entered before CMGF works
	if err := m.ensureSimReady(pin); err != nil {
		return err
	}

	if _, err := m.send("AT+CMGF=1", 2*time.Second); err != nil {
		return fmt.Errorf("text mode failed: %w", err)
	}

	return nil
}

func (m *Modem) Close() {
	if m.port != nil {
		m.port.Close()
	}
}

func (m *Modem) SendSms(number, message string) error {
	resp, err := m.sendExpectPrompt(fmt.Sprintf(`AT+CMGS="%s"`, number), '>', 2*time.Second)
	if err != nil {
		return fmt.Errorf("CMGS prompt failed: %w", err)
	}
	if !strings.Contains(resp, ">") {
		return fmt.Errorf("no prompt received: %s", resp)
	}

	m.port.Write([]byte(message))
	m.port.Write([]byte{26}) // CTRL-Z

	result, err := m.readUntil("OK", 10*time.Second)
	if err != nil {
		return fmt.Errorf("send timeout: %w", err)
	}
	if !strings.Contains(result, "+CMGS") {
		return fmt.Errorf("SMS not confirmed: %s", result)
	}

	slog.Info("SMS sent", "number", number)
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

func (m *Modem) ensureSimReady(pin string) error {
	slog.Info("checking SIM PIN")

	resp, err := m.send("AT+CPIN?", 5*time.Second)
	if err != nil {
		return fmt.Errorf("CPIN query failed: %w", err)
	}

	if strings.Contains(resp, "READY") {
		slog.Info("SIM already ready")
	} else if strings.Contains(resp, "SIM PIN") {
		if pin == "" {
			return fmt.Errorf("SIM requires PIN but none provided")
		}
		slog.Info("sending SIM PIN")
		if _, err := m.send(fmt.Sprintf(`AT+CPIN="%s"`, pin), 5*time.Second); err != nil {
			return fmt.Errorf("PIN send failed: %w", err)
		}
		time.Sleep(5 * time.Second)

		verify, err := m.send("AT+CPIN?", 5*time.Second)
		if err != nil || !strings.Contains(verify, "READY") {
			return fmt.Errorf("PIN not accepted: %s", verify)
		}
		slog.Info("SIM unlocked")
	} else if strings.Contains(resp, "SIM PUK") {
		return fmt.Errorf("SIM blocked (PUK required)")
	} else {
		return fmt.Errorf("unknown CPIN response: %s", resp)
	}

	// Query signal quality
	csq, _ := m.send("AT+CSQ", 3*time.Second)
	slog.Info("signal quality", "response", strings.TrimSpace(csq))

	// Query current operator
	cops, _ := m.send("AT+COPS?", 5*time.Second)
	slog.Info("operator", "response", strings.TrimSpace(cops))

	// Try available operators (scan — can take 30-60s, log for diagnostics)
	slog.Info("waiting for network registration (timeout 2 min)")

	var lastReg string
	deadline := time.Now().Add(2 * time.Minute)
	attempt := 0
	for time.Now().Before(deadline) {
		attempt++

		// Check EPS (LTE/Cat-M1) registration
		reg, err := m.sendWithError("AT+CEREG?", 5*time.Second)
		lastReg = strings.TrimSpace(reg)
		if err == nil && (strings.Contains(reg, ",1") || strings.Contains(reg, ",5")) {
			slog.Info("network registered (CEREG)", "response", lastReg, "attempt", attempt)
			return nil
		}

		// Fallback: check CS (2G/3G) registration
		creg, err2 := m.sendWithError("AT+CREG?", 5*time.Second)
		cregClean := strings.TrimSpace(creg)
		if err2 == nil && (strings.Contains(creg, ",1") || strings.Contains(creg, ",5")) {
			slog.Info("network registered (CREG)", "response", cregClean, "attempt", attempt)
			return nil
		}

		// Log progress every attempt
		slog.Info("network not registered yet",
			"attempt", attempt,
			"CEREG", lastReg,
			"CREG", cregClean,
			"err_cereg", err,
			"err_creg", err2,
		)

		// Re-check signal every 5th attempt
		if attempt%5 == 0 {
			csq, _ := m.send("AT+CSQ", 3*time.Second)
			slog.Info("signal quality (periodic)", "response", strings.TrimSpace(csq))
		}

		time.Sleep(5 * time.Second)
	}

	// Final diagnostics before giving up
	csqFinal, _ := m.send("AT+CSQ", 3*time.Second)
	copsFinal, _ := m.send("AT+COPS?", 5*time.Second)
	slog.Error("network registration failed",
		"last_CEREG", lastReg,
		"CSQ", strings.TrimSpace(csqFinal),
		"COPS", strings.TrimSpace(copsFinal),
		"attempts", attempt,
	)

	return fmt.Errorf("network registration timeout after %d attempts (last CEREG: %s, CSQ: %s)",
		attempt, lastReg, strings.TrimSpace(csqFinal))
}

func (m *Modem) send(cmd string, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	return m.readUntil("OK", timeout)
}

// sendWithError sends a command and accepts both OK and ERROR as valid termination.
// Returns the response and an error only on timeout. If modem returns ERROR, the
// response contains it but err is nil (so caller can inspect the text).
func (m *Modem) sendWithError(cmd string, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	return m.readUntilAny([]string{"OK", "ERROR"}, timeout)
}

func (m *Modem) readUntilAny(expected []string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	var sb strings.Builder
	buf := make([]byte, 256)

	for time.Now().Before(deadline) {
		n, _ := m.port.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			s := sb.String()
			for _, exp := range expected {
				if strings.Contains(s, exp) {
					return s, nil
				}
			}
		}
		if n == 0 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return sb.String(), fmt.Errorf("timeout waiting for %v, got: %s", expected, sb.String())
}

func (m *Modem) sendExpectPrompt(cmd string, prompt byte, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	return m.readUntil(string(prompt), timeout)
}

func (m *Modem) writeLine(cmd string) {
	m.port.Write([]byte(cmd + "\r"))
}

// drain discards all pending data in the serial buffer.
func (m *Modem) drain() {
	buf := make([]byte, 256)
	for {
		n, _ := m.port.Read(buf)
		if n == 0 {
			break
		}
	}
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
