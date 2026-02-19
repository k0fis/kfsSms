package kfs.sc.sms.service;

import kfs.sc.sms.model.SmsMessage;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class SmsRestClient {

    private static final Logger logger = LoggerFactory.getLogger(SmsRestClient.class);

    private final String baseUrl;
    private final String user;
    private final String password;

    public SmsRestClient(String baseUrl, String user, String password) {
        this.baseUrl = baseUrl;
        this.user = user;
        this.password = password;
    }

    public void send(SmsMessage msg) {
    }


}
