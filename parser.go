package main

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

type SmsMessage struct {
	Index     int
	Status    string
	Sender    string
	Timestamp time.Time
	Text      string
}

// ParseCMGL parses AT+CMGL="ALL" response into a slice of SMS messages.
func ParseCMGL(response string) []SmsMessage {
	if response == "" {
		return nil
	}

	lines := strings.Split(response, "\n")
	var messages []SmsMessage
	var current *SmsMessage
	var body strings.Builder

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")

		if strings.HasPrefix(line, "+CMGL:") {
			// Save previous message
			if current != nil {
				current.Text = decodeText(strings.TrimSpace(body.String()))
				messages = append(messages, *current)
			}

			// Parse header
			current = parseHeader(line)
			body.Reset()
		} else if line == "OK" || line == "ERROR" || strings.HasPrefix(line, "+CMS ERROR") {
			break
		} else if current != nil {
			if body.Len() > 0 {
				body.WriteByte('\n')
			}
			body.WriteString(line)
		}
	}

	// Save last message
	if current != nil {
		current.Text = decodeText(strings.TrimSpace(body.String()))
		messages = append(messages, *current)
	}

	return messages
}

// parseHeader parses a +CMGL header line.
// Format: +CMGL: index,"status","sender","","timestamp"
func parseHeader(line string) *SmsMessage {
	// Strip "+CMGL: " prefix
	rest := strings.TrimPrefix(line, "+CMGL: ")
	if rest == line {
		return nil
	}

	fields := splitCSV(rest)
	if len(fields) < 5 {
		return nil
	}

	index, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil {
		return nil
	}

	status := strings.Trim(fields[1], "\" ")
	sender := strings.Trim(fields[2], "\" ")
	// fields[3] is empty (name)
	tsRaw := strings.Trim(fields[4], "\" ")

	ts := parseTimestamp(tsRaw)

	return &SmsMessage{
		Index:     index,
		Status:    status,
		Sender:    sender,
		Timestamp: ts,
	}
}

// splitCSV splits a string by commas, respecting quoted fields.
func splitCSV(input string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for _, c := range input {
		switch {
		case c == '"':
			inQuotes = !inQuotes
			current.WriteRune(c)
		case c == ',' && !inQuotes:
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(c)
		}
	}
	fields = append(fields, current.String())
	return fields
}

// parseTimestamp parses modem timestamp format: yy/MM/dd,HH:mm:ss±ZZ
func parseTimestamp(raw string) time.Time {
	// Examples: "24/02/12,09:41:22+04" or "25/01/15,14:30:00+01"
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now()
	}

	// Normalize timezone: "+04" → "+04:00"
	if len(raw) >= 2 {
		suffix := raw[len(raw)-3:]
		if (suffix[0] == '+' || suffix[0] == '-') && len(suffix) == 3 {
			raw = raw + ":00"
		}
	}

	t, err := time.Parse("06/01/02,15:04:05-07:00", raw)
	if err != nil {
		return time.Now()
	}
	return t
}

// decodeText detects UCS-2 hex-encoded SMS text and decodes it to UTF-8.
// Modem sends UCS-2 hex when SMS contains non-ASCII characters (e.g. Czech diacritics).
// Example: "00410068006F006A0020010D006100750020" → "Ahoj čau "
func decodeText(text string) string {
	slog.Info("decodeText input", "len", len(text), "text", text, "isUCS2", isUCS2Hex(text))
	if !isUCS2Hex(text) {
		return text
	}

	b, err := hex.DecodeString(text)
	if err != nil {
		return text
	}

	// Decode UTF-16BE to runes
	if len(b)%2 != 0 {
		return text
	}

	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(b); i += 2 {
		u16[i/2] = uint16(b[i])<<8 | uint16(b[i+1])
	}

	runes := utf16.Decode(u16)
	return strings.TrimRight(string(runes), "\x00 ")
}

// isUCS2Hex checks if a string looks like UCS-2 hex encoding:
// - only hex chars (0-9, A-F, a-f)
// - length is multiple of 4
// - at least 8 chars (2 UCS-2 characters minimum)
func isUCS2Hex(s string) bool {
	if len(s) < 8 || len(s)%4 != 0 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// needsUCS2 returns true if the string contains any non-ASCII character.
func needsUCS2(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// stripDiacritics replaces accented characters with their ASCII equivalents.
// Used for long messages (>70 chars) where UCS-2 concatenation isn't supported.
func stripDiacritics(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if repl, ok := diacriticMap[r]; ok {
			sb.WriteRune(repl)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var diacriticMap = map[rune]rune{
	'á': 'a', 'Á': 'A', 'à': 'a', 'À': 'A', 'â': 'a', 'Â': 'A', 'ä': 'a', 'Ä': 'A', 'ã': 'a', 'Ã': 'A',
	'č': 'c', 'Č': 'C', 'ć': 'c', 'Ć': 'C', 'ç': 'c', 'Ç': 'C',
	'ď': 'd', 'Ď': 'D',
	'é': 'e', 'É': 'E', 'è': 'e', 'È': 'E', 'ê': 'e', 'Ê': 'E', 'ě': 'e', 'Ě': 'E', 'ë': 'e', 'Ë': 'E',
	'í': 'i', 'Í': 'I', 'î': 'i', 'Î': 'I', 'ï': 'i', 'Ï': 'I',
	'ľ': 'l', 'Ľ': 'L', 'ĺ': 'l', 'Ĺ': 'L', 'ł': 'l', 'Ł': 'L',
	'ň': 'n', 'Ň': 'N', 'ń': 'n', 'Ń': 'N', 'ñ': 'n', 'Ñ': 'N',
	'ó': 'o', 'Ó': 'O', 'ô': 'o', 'Ô': 'O', 'ö': 'o', 'Ö': 'O', 'õ': 'o', 'Õ': 'O',
	'ř': 'r', 'Ř': 'R', 'ŕ': 'r', 'Ŕ': 'R',
	'š': 's', 'Š': 'S', 'ś': 's', 'Ś': 'S',
	'ť': 't', 'Ť': 'T',
	'ú': 'u', 'Ú': 'U', 'ů': 'u', 'Ů': 'U', 'ü': 'u', 'Ü': 'U', 'ù': 'u', 'Ù': 'U', 'û': 'u', 'Û': 'U',
	'ý': 'y', 'Ý': 'Y', 'ÿ': 'y',
	'ž': 'z', 'Ž': 'Z', 'ź': 'z', 'Ź': 'Z', 'ż': 'z', 'Ż': 'Z',
}

// splitMessage splits a string into chunks of at most maxRunes runes.
// Tries to break at word boundaries when possible.
func splitMessage(s string, maxRunes int) []string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return []string{s}
	}

	var parts []string
	for len(runes) > 0 {
		end := maxRunes
		if end > len(runes) {
			end = len(runes)
		}

		// Try to break at a space within last 20% of the chunk
		if end < len(runes) {
			minBreak := end - end/5
			bestBreak := -1
			for i := end - 1; i >= minBreak; i-- {
				if runes[i] == ' ' {
					bestBreak = i + 1 // include the space in the current chunk
					break
				}
			}
			if bestBreak > 0 {
				end = bestBreak
			}
		}

		parts = append(parts, string(runes[:end]))
		runes = runes[end:]
	}
	return parts
}

// encodeUCS2Hex encodes a Go string to UCS-2 (UTF-16BE) hex representation.
func encodeUCS2Hex(s string) string {
	runes := []rune(s)
	u16 := utf16.Encode(runes)
	var sb strings.Builder
	for _, v := range u16 {
		sb.WriteString(fmt.Sprintf("%04X", v))
	}
	return sb.String()
}
