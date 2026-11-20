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
		port.SetDTR(true)
		port.SetRTS(true)

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

	// Retry opening modem — on Linux the USB serial port may need multiple
	// attempts before the modem responds to AT commands (driver timing).
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		port, err := serial.Open(m.portName, mode)
		if err != nil {
			return fmt.Errorf("cannot open port %s: %w", m.portName, err)
		}
		port.SetReadTimeout(200 * time.Millisecond)
		port.SetDTR(true)
		port.SetRTS(true)

		// Flush stale data
		buf := make([]byte, 256)
		for {
			n, _ := port.Read(buf)
			if n == 0 {
				break
			}
		}

		// Wake modem with bare CR, then try AT
		port.Write([]byte("\r"))
		time.Sleep(300 * time.Millisecond)
		for {
			n, _ := port.Read(buf)
			if n == 0 {
				break
			}
		}

		port.Write([]byte("AT\r"))
		time.Sleep(1 * time.Second)

		var resp strings.Builder
		for {
			n, _ := port.Read(buf)
			if n == 0 {
				break
			}
			resp.Write(buf[:n])
		}

		if strings.Contains(resp.String(), "OK") {
			// AT works — now send ATE0 inline
			port.Write([]byte("ATE0\r"))
			time.Sleep(500 * time.Millisecond)
			for {
				n, _ := port.Read(buf)
				if n == 0 {
					break
				}
			}

			m.port = port
			m.port.SetReadTimeout(100 * time.Millisecond)
			slog.Info("modem ready", "attempt", attempt)
			goto modemReady
		}

		slog.Warn("modem not responding, retrying", "attempt", attempt, "got", resp.String())
		port.Close()
		lastErr = fmt.Errorf("attempt %d: no OK, got: %s", attempt, resp.String())
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("modem handshake failed after 5 attempts: %w", lastErr)

modemReady:

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
	if needsUCS2(message) {
		return m.sendSmsPDU(number, message)
	}
	return m.sendSmsText(number, message)
}

// sendSmsText sends an ASCII-only SMS in text mode (AT+CMGF=1).
// The modem handles concatenation for messages >160 chars.
func (m *Modem) sendSmsText(number, message string) error {
	resp, err := m.sendExpectPrompt(fmt.Sprintf(`AT+CMGS="%s"`, number), '>', 2*time.Second)
	if err != nil {
		return fmt.Errorf("CMGS prompt failed: %w", err)
	}
	if !strings.Contains(resp, ">") {
		return fmt.Errorf("no prompt received: %s", resp)
	}
	m.port.Write([]byte(message))
	m.port.Write([]byte{26}) // CTRL-Z

	return m.waitSendResult()
}

// sendSmsPDU sends a UCS-2 SMS using PDU mode (AT+CMGF=0).
// Handles concatenation via UDH for messages >70 chars.
func (m *Modem) sendSmsPDU(number, message string) error {
	// Switch to PDU mode
	if _, err := m.send("AT+CMGF=0", 2*time.Second); err != nil {
		return fmt.Errorf("switch to PDU mode failed: %w", err)
	}

	segments := splitUCS2Message(message)
	total := byte(len(segments))
	ref := byte(time.Now().UnixNano() & 0xFF)

	slog.Info("sending UCS2 via PDU", "segments", total, "chars", len([]rune(message)))

	var sendErr error
	for i, seg := range segments {
		part := byte(i + 1)
		pduHex, tpduLen := BuildSMSSubmitPDU(number, seg, ref, total, part)

		slog.Info("PDU segment prepare", "part", part, "total", total,
			"tpduLen", tpduLen, "pduHexLen", len(pduHex), "segChars", len([]rune(seg)))

		// Drain stale data before each segment
		m.drain()

		// Verify modem is responsive before sending
		if _, err := m.send("AT", 2*time.Second); err != nil {
			sendErr = fmt.Errorf("modem not ready before part %d/%d: %w", part, total, err)
			break
		}

		// Re-confirm PDU mode (modem may have reset after error/timeout)
		if _, err := m.send("AT+CMGF=0", 2*time.Second); err != nil {
			sendErr = fmt.Errorf("CMGF=0 failed before part %d/%d: %w", part, total, err)
			break
		}

		resp, err := m.sendExpectPrompt(fmt.Sprintf("AT+CMGS=%d", tpduLen), '>', 5*time.Second)
		if err != nil {
			sendErr = fmt.Errorf("CMGS prompt failed (part %d/%d): %w", part, total, err)
			break
		}
		if !strings.Contains(resp, ">") {
			sendErr = fmt.Errorf("no prompt (part %d/%d): %s", part, total, resp)
			break
		}

		m.port.Write([]byte(pduHex))
		m.port.Write([]byte{26}) // CTRL-Z

		if err := m.waitSendResult(); err != nil {
			sendErr = fmt.Errorf("part %d/%d failed: %w", part, total, err)
			break
		}

		slog.Info("PDU segment sent", "part", part, "total", total)

		// Delay between segments — modem needs time for radio TX + network ACK
		if i < len(segments)-1 {
			time.Sleep(3 * time.Second)
		}
	}

	// Recover modem: ESC (cancel any stuck prompt) + drain
	m.port.Write([]byte{27}) // ESC
	time.Sleep(500 * time.Millisecond)
	m.drain()

	// Switch back to text mode (for incoming SMS reading via CMGL)
	if _, err := m.send("AT+CMGF=1", 3*time.Second); err != nil {
		slog.Warn("switch to text mode failed, retrying after AT reset", "err", err)
		m.port.Write([]byte("\r"))
		time.Sleep(300 * time.Millisecond)
		m.drain()
		m.send("AT", 2*time.Second)
		if _, err := m.send("AT+CMGF=1", 3*time.Second); err != nil {
			slog.Error("switch to text mode failed after retry", "err", err)
		}
	}

	if sendErr != nil {
		return sendErr
	}

	slog.Info("SMS sent", "number", number, "ucs2", true, "segments", total)
	return nil
}

// waitSendResult reads the modem response after CTRL-Z and checks for errors.
func (m *Modem) waitSendResult() error {
	result, err := m.readUntilAny([]string{"OK", "ERROR"}, 15*time.Second)
	if err != nil {
		return fmt.Errorf("send timeout: %w", err)
	}
	result = strings.TrimSpace(result)

	slog.Debug("waitSendResult raw", "result", result)

	// Check for CMS/CME ERROR first — modem may include trailing OK after error
	if strings.Contains(result, "+CMS ERROR") || strings.Contains(result, "+CME ERROR") {
		m.drain()
		return fmt.Errorf("SMS send error: %s", result)
	}

	if !strings.Contains(result, "+CMGS") {
		if !strings.Contains(result, "OK") {
			slog.Warn("SMS not confirmed, raw response", "result", fmt.Sprintf("%q", result))
			return fmt.Errorf("SMS not confirmed: %s", result)
		}
		slog.Warn("SMS sent (bare OK, no +CMGS ref)", "response", result)
	}

	return nil
}

func (m *Modem) ReadAll() ([]SmsMessage, error) {
	resp, err := m.send("AT+CMGL=\"ALL\"", 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("CMGL failed: %w", err)
	}
	if strings.Contains(resp, "+CMGL:") {
		slog.Info("CMGL raw response", "len", len(resp), "resp", resp)
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
	resp, err := m.readUntil("OK", timeout)
	return strings.TrimSpace(resp), err
}

// sendWithError sends a command and accepts both OK and ERROR as valid termination.
// Returns the response and an error only on timeout. If modem returns ERROR, the
// response contains it but err is nil (so caller can inspect the text).
func (m *Modem) sendWithError(cmd string, timeout time.Duration) (string, error) {
	m.writeLine(cmd)
	resp, err := m.readUntilAny([]string{"OK", "ERROR"}, timeout)
	return strings.TrimSpace(resp), err
}

func (m *Modem) readUntil(expected string, timeout time.Duration) (string, error) {
	return m.readUntilAny([]string{expected}, timeout)
}

func (m *Modem) readUntilAny(expected []string, timeout time.Duration) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 256)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, err := m.port.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			s := sb.String()
			for _, exp := range expected {
				if strings.Contains(s, exp) {
					return s, nil
				}
			}
		}
		if err != nil && n == 0 {
			// Real error (not just timeout with no data)
			if err.Error() != "EOF" {
				return sb.String(), fmt.Errorf("read error: %w", err)
			}
		}
		// n==0, no error — port read timeout elapsed, loop and retry until deadline
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

// drain discards all pending data in the serial buffer (with hard 500ms limit).
func (m *Modem) drain() {
	deadline := time.Now().Add(500 * time.Millisecond)
	buf := make([]byte, 256)
	for time.Now().Before(deadline) {
		n, _ := m.port.Read(buf)
		if n == 0 {
			return
		}
	}
}
