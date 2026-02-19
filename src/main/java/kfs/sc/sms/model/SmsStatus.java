package kfs.sc.sms.model;

public enum SmsStatus {
    REC_READ,
    REC_UNREAD,
    STO_SENT,
    STO_UNSENT;

    public static SmsStatus fromModemValue(String value) {
        return SmsStatus.valueOf(value.replace(' ', '_'));
    }
}
