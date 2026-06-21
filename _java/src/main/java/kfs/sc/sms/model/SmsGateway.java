package kfs.sc.sms.model;

import kfs.sc.sms.utils.ModemException;

import java.util.List;

public interface SmsGateway {

    void open(String pin) throws ModemException;

    void close();

    void sendSms(String number, String message) throws ModemException;

    List<SmsMessage> readAll() throws ModemException;

    void delete(int index) throws ModemException;
}
