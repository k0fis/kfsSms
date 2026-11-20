package main

import (
	"strings"
	"testing"
)

func TestEncodeBCD(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"420603190450", "246030914005"},
		{"42060319045", "2460309140F5"}, // odd length → padded with F
		{"1234", "2143"},
	}
	for _, tt := range tests {
		got := encodeBCD(tt.input)
		if got != tt.want {
			t.Errorf("encodeBCD(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSplitUCS2Message(t *testing.T) {
	// Short message — single segment
	short := "Ahoj"
	segs := splitUCS2Message(short)
	if len(segs) != 1 {
		t.Errorf("short message: got %d segments, want 1", len(segs))
	}

	// Exactly 70 chars — single segment
	exact70 := strings.Repeat("á", 70)
	segs = splitUCS2Message(exact70)
	if len(segs) != 1 {
		t.Errorf("70 char message: got %d segments, want 1", len(segs))
	}

	// 71 chars — 2 segments (67 + 4)
	over70 := strings.Repeat("á", 71)
	segs = splitUCS2Message(over70)
	if len(segs) != 2 {
		t.Errorf("71 char message: got %d segments, want 2", len(segs))
	}
	if len([]rune(segs[0])) != 67 {
		t.Errorf("first segment: got %d runes, want 67", len([]rune(segs[0])))
	}
	if len([]rune(segs[1])) != 4 {
		t.Errorf("second segment: got %d runes, want 4", len([]rune(segs[1])))
	}

	// ~350 chars (like the real SMS) — 6 segments
	long := strings.Repeat("č", 350)
	segs = splitUCS2Message(long)
	// 350 / 67 = 5.22 → 6 segments
	if len(segs) != 6 {
		t.Errorf("350 char message: got %d segments, want 6", len(segs))
	}
}

func TestBuildSMSSubmitPDU_Single(t *testing.T) {
	pdu, tpduLen := BuildSMSSubmitPDU("+420603190450", "Ahoj", 0, 1, 1)

	// SCA = "00"
	if !strings.HasPrefix(pdu, "00") {
		t.Error("PDU should start with SCA 00")
	}

	// PDU type should be 01 (no UDHI for single)
	if pdu[2:4] != "01" {
		t.Errorf("PDU type = %s, want 01", pdu[2:4])
	}

	// Should contain UCS-2 encoded "Ahoj" = 0041 0068 006F 006A
	if !strings.Contains(strings.ToUpper(pdu), "0041006800") {
		t.Error("PDU should contain UCS-2 encoded 'Ah'")
	}

	// TPDU length should be positive
	if tpduLen <= 0 {
		t.Errorf("tpduLen = %d, want > 0", tpduLen)
	}

	// Verify TPDU length matches: (len(pdu) - 2 for "00" SCA) / 2
	expectedTPDULen := (len(pdu) - 2) / 2
	if tpduLen != expectedTPDULen {
		t.Errorf("tpduLen = %d, want %d", tpduLen, expectedTPDULen)
	}
}

func TestBuildSMSSubmitPDU_Concatenated(t *testing.T) {
	pdu, _ := BuildSMSSubmitPDU("+420603190450", "část", 0xAB, 3, 1)

	// PDU type should be 41 (UDHI set)
	if pdu[2:4] != "41" {
		t.Errorf("PDU type = %s, want 41 (UDHI)", pdu[2:4])
	}

	// Should contain UDH: 05 00 03 AB 03 01
	if !strings.Contains(strings.ToUpper(pdu), "050003AB0301") {
		t.Errorf("PDU missing concat UDH, pdu=%s", pdu)
	}
}

func TestBuildSMSSubmitPDU_DCS(t *testing.T) {
	pdu, _ := BuildSMSSubmitPDU("+420123456789", "x", 0, 1, 1)

	// Find DCS byte: after DA (variable), PID (00), then DCS should be 08
	// The PDU structure after "00" SCA:
	// 01(type) 00(MR) 0C(DA-len=12digits) 91(TON) XXXXXX(BCD 6 bytes) 00(PID) 08(DCS) ...
	// Position: 2 + 2 + 2 + 2 + 2 + 12 + 2 = 24, then DCS at 24..26
	if pdu[24:26] != "08" {
		t.Errorf("DCS = %s, want 08 (UCS-2), full pdu=%s", pdu[24:26], pdu)
	}
}
