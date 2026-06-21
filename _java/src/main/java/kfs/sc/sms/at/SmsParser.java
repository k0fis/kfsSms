package kfs.sc.sms.at;

import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.model.SmsStatus;

import java.time.OffsetDateTime;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

public class SmsParser {

    private static final DateTimeFormatter MODEM_FORMAT =
            DateTimeFormatter.ofPattern("yy/MM/dd,HH:mm:ssXXX");

    public static List<SmsMessage> parseCmglResponse(List<String> lines) {

        List<SmsMessage> messages = new ArrayList<>();

        ParsedHeader currentHeader = null;
        StringBuilder bodyBuilder = new StringBuilder();

        for (String rawLine : lines) {

            String line = rawLine == null ? "" : rawLine.trim();

            // Hard stop on ERROR
            if ("ERROR".equals(line) || line.startsWith("+CMS ERROR")) {
                return List.of();
            }

            if (line.startsWith("+CMGL:")) {

                // uložíme předchozí zprávu
                if (currentHeader != null) {
                    buildSafely(messages, currentHeader, bodyBuilder);
                    bodyBuilder.setLength(0);
                }

                currentHeader = parseHeaderSafely(line);
                continue;
            }

            if ("OK".equals(line)) {
                break;
            }

            if (currentHeader == null) {
                continue;
            }

            if (!bodyBuilder.isEmpty()) {
                bodyBuilder.append("\n");
            }

            bodyBuilder.append(rawLine == null ? "" : rawLine); // zachováme originální whitespace
        }

        if (currentHeader != null) {
            buildSafely(messages, currentHeader, bodyBuilder);
        }

        return messages;
    }


    private static void buildSafely(List<SmsMessage> messages,
                                    ParsedHeader header,
                                    StringBuilder bodyBuilder) {
        try {
            SmsMessage message = new SmsMessage(
                    header.index(),
                    header.status(),
                    header.sender(),
                    Optional.ofNullable(header.timestamp()).orElse(OffsetDateTime.now()),
                    bodyBuilder.toString()
            );
            messages.add(message);
        } catch (Exception ignored) {
            // logovat ve vyšší vrstvě pokud chceš
        }
    }


    private static ParsedHeader parseHeaderSafely(String line) {
        try {
            return parseHeader(line);
        } catch (Exception e) {
            return null;
        }
    }



    private static ParsedHeader parseHeader(String line) {

        // remove "+CMGL: "
        String content = line.substring(7).trim();

        // split respecting quotes
        List<String> parts = splitCsvRespectingQuotes(content);

        int index = Integer.parseInt(parts.get(0));
        SmsStatus status = SmsStatus.fromModemValue(stripQuotes(parts.get(1)));
        String sender = stripQuotes(parts.get(2));
        String timestampRaw = stripQuotes(parts.get(4));
        OffsetDateTime timestamp = parseTimestamp(timestampRaw);

        return new ParsedHeader(index, status, sender, timestamp);
    }

    private static OffsetDateTime parseTimestamp(String raw) {
        try {
            String normalized = raw.replaceAll("(\\+|\\-)(\\d{2})$", "$1$2:00");
            return OffsetDateTime.parse(normalized, MODEM_FORMAT);
        } catch (Exception e) {
            return null;
        }
    }


    private static String stripQuotes(String s) {
        if (s == null) return null;
        return s.replaceAll("^\"|\"$", "");
    }

    private static List<String> splitCsvRespectingQuotes(String input) {

        List<String> result = new ArrayList<>();
        StringBuilder current = new StringBuilder();
        boolean inQuotes = false;

        for (char c : input.toCharArray()) {

            if (c == '"') {
                inQuotes = !inQuotes;
            }

            if (c == ',' && !inQuotes) {
                result.add(current.toString());
                current.setLength(0);
            } else {
                current.append(c);
            }
        }

        result.add(current.toString());
        return result;
    }

    private record ParsedHeader(
            int index,
            SmsStatus status,
            String sender,
            OffsetDateTime timestamp
    ) {}
}
