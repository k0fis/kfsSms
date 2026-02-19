package kfs.sc.sms.at;

import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.model.SmsStatus;
import org.junit.jupiter.api.Test;

import java.time.OffsetDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class SmsParserTest {

    @Test
    void shouldParseSingleMessage() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"",
                "Ahoj",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(1, messages.size());

        SmsMessage msg = messages.get(0);

        assertEquals(1, msg.getIndex());
        assertEquals(SmsStatus.REC_READ, msg.getStatus());
        assertEquals("+420123456789", msg.getSender());
        assertEquals("Ahoj", msg.getBody());

        OffsetDateTime expected =
                OffsetDateTime.parse("2024-02-12T09:41:22+04:00");

        assertEquals(expected, msg.getTimestamp());
    }

    @Test
    void shouldParseMultipleMessages() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420111111111\",\"\",\"24/02/12,09:41:22+04\"",
                "Prvni",
                "",
                "+CMGL: 2,\"REC UNREAD\",\"+420222222222\",\"\",\"24/02/12,10:00:00+04\"",
                "Druha",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(2, messages.size());

        assertEquals("Prvni\n", messages.get(0).getBody());
        assertEquals("Druha", messages.get(1).getBody());
    }

    @Test
    void shouldParseMultilineBody() {

        List<String> lines = List.of(
                "+CMGL: 3,\"REC READ\",\"+420333333333\",\"\",\"24/02/12,09:50:01+04\"",
                "Prvni radek",
                "Druhy radek",
                "Treti radek",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(1, messages.size());

        String expectedBody =
                "Prvni radek\nDruhy radek\nTreti radek";

        assertEquals(expectedBody, messages.get(0).getBody());
    }

    @Test
    void shouldIgnoreLinesBeforeFirstHeader() {

        List<String> lines = List.of(
                "",
                "RANDOM",
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"",
                "Body",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(1, messages.size());
        assertEquals("Body", messages.get(0).getBody());
    }

    @Test
    void shouldReturnEmptyListWhenNoMessages() {

        List<String> lines = List.of(
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertTrue(messages.isEmpty());
    }

    @Test
    void shouldReturnEmptyListWhenErrorOccurs() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"",
                "Body",
                "ERROR"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertTrue(messages.isEmpty());
    }


    @Test
    void shouldReturnEmptyListWhenCmsErrorOccurs() {

        List<String> lines = List.of(
                "+CMS ERROR: 500"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertTrue(messages.isEmpty());
    }


    @Test
    void shouldParseMessageEvenWithoutOkTerminator() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"",
                "Body"
                // žádné OK
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(1, messages.size());
    }

    @Test
    void shouldFailWhenHeaderIsBroken() {

        List<String> lines = List.of(
                "+CMGL: BROKEN HEADER",
                "Body",
                "OK"
        );
        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(0, messages.size());
    }


    @Test
    void shouldThrowWhenTimestampIsInvalid() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"BAD_TIMESTAMP\"",
                "Body",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(1, messages.size());
    }

    @Test
    void shouldSkipBrokenMessageAndParseValidOne() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"BAD_TIMESTAMP\"",
                "Broken",
                "+CMGL: 2,\"REC READ\",\"+420111111111\",\"\",\"24/02/12,10:00:00+04\"",
                "Valid",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertEquals(2, messages.size());
        assertEquals("Valid", messages.get(1).getBody());
    }


    @Test
    void shouldSkipMessageWithIncompleteHeader() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\"",
                "Body",
                "OK"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertTrue(messages.isEmpty());
    }
    @Test
    void shouldReturnEmptyListWhenErrorOccursAfterMessage() {

        List<String> lines = List.of(
                "+CMGL: 1,\"REC READ\",\"+420123456789\",\"\",\"24/02/12,09:41:22+04\"",
                "Body",
                "ERROR"
        );

        List<SmsMessage> messages = SmsParser.parseCmglResponse(lines);

        assertTrue(messages.isEmpty());
    }

}
