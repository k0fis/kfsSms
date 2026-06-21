package kfs.sc.sms.model;

import kfs.sc.sms.utils.ModemException;

import java.time.Duration;

public interface ModemSession {

    void start();

    void stop();

    boolean isReady();

    String execute(String command, Duration timeout) throws ModemException;
}
