package main

import (
	"testing"
	"time"
)

func TestParseCMGL_Empty(t *testing.T) {
	msgs := ParseCMGL("")
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

func TestParseCMGL_Single(t *testing.T) {
	response := "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"\r\nHello world\r\n\r\nOK\r\n"
	msgs := ParseCMGL(response)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]
	if m.Index != 1 {
		t.Errorf("index = %d, want 1", m.Index)
	}
	if m.Status != "REC READ" {
		t.Errorf("status = %q, want REC READ", m.Status)
	}
	if m.Sender != "+420123456789" {
		t.Errorf("sender = %q", m.Sender)
	}
	if m.Text != "Hello world" {
		t.Errorf("text = %q, want Hello world", m.Text)
	}
	if m.Timestamp.Year() != 2024 || m.Timestamp.Month() != 2 || m.Timestamp.Day() != 12 {
		t.Errorf("timestamp = %v", m.Timestamp)
	}
}

func TestParseCMGL_Multiple(t *testing.T) {
	response := "+CMGL: 0,\"REC UNREAD\",\"+420111222333\",\"\",\"25/06/15,10:00:00+02\"\r\nFirst\r\n+CMGL: 1,\"REC READ\",\"+420444555666\",\"\",\"25/06/15,11:00:00+02\"\r\nSecond message\r\nwith newline\r\n\r\nOK\r\n"
	msgs := ParseCMGL(response)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Text != "First" {
		t.Errorf("msg[0].text = %q", msgs[0].Text)
	}
	if msgs[1].Text != "Second message\nwith newline" {
		t.Errorf("msg[1].text = %q", msgs[1].Text)
	}
	if msgs[1].Sender != "+420444555666" {
		t.Errorf("msg[1].sender = %q", msgs[1].Sender)
	}
}

func TestParseCMGL_Error(t *testing.T) {
	response := "+CMS ERROR: 500\r\n"
	msgs := ParseCMGL(response)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages on error, got %d", len(msgs))
	}
}

func TestParseTimestamp(t *testing.T) {
	ts := parseTimestamp("24/02/12,09:41:22+04")
	if ts.Year() != 2024 || ts.Month() != time.February || ts.Day() != 12 {
		t.Errorf("date = %v", ts)
	}
	if ts.Hour() != 9 || ts.Minute() != 41 || ts.Second() != 22 {
		t.Errorf("time = %v", ts)
	}
}

func TestParseTimestamp_Negative(t *testing.T) {
	ts := parseTimestamp("25/12/31,23:59:59-05")
	if ts.Year() != 2025 || ts.Month() != time.December {
		t.Errorf("date = %v", ts)
	}
}

func TestSplitCSV(t *testing.T) {
	fields := splitCSV(`1,"REC READ","+420123","","24/02/12,09:41:22+04"`)
	if len(fields) != 5 {
		t.Fatalf("expected 5 fields, got %d: %v", len(fields), fields)
	}
	if fields[0] != "1" {
		t.Errorf("field[0] = %q", fields[0])
	}
}
