package main

import (
	"fmt"
	"strings"
	"unicode/utf16"
)

// SMS-SUBMIT PDU builder for UCS-2 concatenated messages.
// Reference: 3GPP TS 23.040 (SMS), TS 23.038 (encoding), TS 24.011 (transport)

const (
	// maxUCS2SingleChars is the max UCS-2 characters in a single (non-concatenated) SMS.
	maxUCS2SingleChars = 70 // 140 bytes / 2 bytes per char

	// maxUCS2ConcatChars is the max UCS-2 characters per segment with 8-bit concat UDH (6 bytes).
	maxUCS2ConcatChars = 67 // (140 - 6) / 2
)

// splitUCS2Message splits a message into segments suitable for concatenated UCS-2 SMS.
func splitUCS2Message(text string) []string {
	runes := []rune(text)
	if len(runes) <= maxUCS2SingleChars {
		return []string{text}
	}

	var segments []string
	for i := 0; i < len(runes); i += maxUCS2ConcatChars {
		end := i + maxUCS2ConcatChars
		if end > len(runes) {
			end = len(runes)
		}
		segments = append(segments, string(runes[i:end]))
	}
	return segments
}

// BuildSMSSubmitPDU builds an SMS-SUBMIT PDU in hex for UCS-2 encoding.
// For concatenated messages, pass ref (same for all parts), total parts, and part number (1-based).
// For single messages, pass total=1, part=1 (UDH is omitted).
// Returns the full PDU hex string and the TPDU length (octets, excluding SCA).
func BuildSMSSubmitPDU(number, text string, ref, total, part byte) (pduHex string, tpduLen int) {
	var pdu strings.Builder

	// SCA (Service Center Address): 00 = use default from SIM
	pdu.WriteString("00")

	tpduStart := pdu.Len()

	// First octet of SMS-SUBMIT
	// Bits: RP=0, UDHI, SRR=0, VPF=00(none), RD=0, MTI=01(submit)
	if total > 1 {
		pdu.WriteString("41") // 0100 0001 = UDHI set + SMS-SUBMIT
	} else {
		pdu.WriteString("01") // 0000 0001 = SMS-SUBMIT
	}

	// TP-MR (Message Reference): 00 = let modem assign
	pdu.WriteString("00")

	// TP-DA (Destination Address)
	num := strings.TrimPrefix(number, "+")
	pdu.WriteString(fmt.Sprintf("%02X", len(num))) // address length (digits)
	pdu.WriteString("91")                          // TON/NPI: international
	pdu.WriteString(encodeBCD(num))

	// TP-PID (Protocol Identifier): 00 = default
	pdu.WriteString("00")

	// TP-DCS (Data Coding Scheme): 08 = UCS-2
	pdu.WriteString("08")

	// No TP-VP (Validity Period) since VPF=00

	// TP-UD (User Data)
	runes := []rune(text)
	u16 := utf16.Encode(runes)
	msgBytes := len(u16) * 2

	if total > 1 {
		// UDL = UDH length (6 bytes) + message bytes
		pdu.WriteString(fmt.Sprintf("%02X", 6+msgBytes))
		// UDH: concatenated SMS, 8-bit reference
		pdu.WriteString("05")                          // UDHL (UDH length excluding itself)
		pdu.WriteString("00")                          // IEI: concatenated SMS, 8-bit ref
		pdu.WriteString("03")                          // IEI data length
		pdu.WriteString(fmt.Sprintf("%02X", ref))      // reference number
		pdu.WriteString(fmt.Sprintf("%02X", total))    // total segments
		pdu.WriteString(fmt.Sprintf("%02X", part))     // segment number
	} else {
		// UDL = message bytes only
		pdu.WriteString(fmt.Sprintf("%02X", msgBytes))
	}

	// Message body as UTF-16BE hex
	for _, v := range u16 {
		pdu.WriteString(fmt.Sprintf("%04X", v))
	}

	fullPDU := pdu.String()
	// TPDU length = total hex chars after SCA, divided by 2 (hex chars to bytes)
	tpduLen = (len(fullPDU) - tpduStart) / 2

	return fullPDU, tpduLen
}

// encodeBCD encodes a digit string to swapped-nibble BCD used in GSM addresses.
// Odd-length numbers are padded with F. Example: "420603190450" → "240630910450"
func encodeBCD(num string) string {
	if len(num)%2 != 0 {
		num += "F"
	}
	var sb strings.Builder
	for i := 0; i < len(num); i += 2 {
		sb.WriteByte(num[i+1])
		sb.WriteByte(num[i])
	}
	return sb.String()
}
