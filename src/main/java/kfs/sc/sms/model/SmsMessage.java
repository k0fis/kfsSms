package kfs.sc.sms.model;

import java.time.LocalDateTime;
import java.time.OffsetDateTime;

public record SmsMessage(
        int index,
        SmsStatus status,
        String sender,
        OffsetDateTime timestamp,
        String text
) {

    public int getIndex() {
        return index;
    }

    public SmsStatus getStatus() {
        return status;
    }

    public String getSender() {
        return sender;
    }

    public String getBody() {
        return text;
    }

    public OffsetDateTime getTimestamp() {
        return timestamp;
    }

    @Override
    public String toString() {
        return "SmsMessage{" +
                "index=" + index +
                ", status=" + status +
                ", sender='" + sender + '\'' +
                ", timestamp=" + timestamp +
                ", text='" + text + '\'' +
                '}';
    }
}
