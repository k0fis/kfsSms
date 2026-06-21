package kfs.sc.sms.utils;

public class KfsSmsException extends RuntimeException {

    public KfsSmsException(String message) {
        super(message);
    }


    public KfsSmsException(String message, Throwable throwable) {
        super(message, throwable);
    }
}
