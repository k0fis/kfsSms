package main

import (
	"strconv"
	"strings"
	"time"
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
				current.Text = strings.TrimSpace(body.String())
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
		current.Text = strings.TrimSpace(body.String())
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
