package kfs.sc.sms.utils;

public class ModemException extends Exception {

    public ModemException(String message) {
        super(message);
    }


    public ModemException(String message, Throwable throwable) {
        super(message, throwable);
    }
}
