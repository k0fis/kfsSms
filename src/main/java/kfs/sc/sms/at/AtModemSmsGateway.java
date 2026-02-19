package kfs.sc.sms.at;

import com.fazecast.jSerialComm.SerialPort;
import kfs.sc.sms.model.SmsGateway;
import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.utils.ModemException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.List;

public class AtModemSmsGateway implements SmsGateway {

    private static final Logger logger = LoggerFactory.getLogger(AtModemSmsGateway.class);

    private final String portName;
    private SerialPort port;
    private AtCommandExecutor executor;

    public AtModemSmsGateway(String portName) {
        this.portName = portName;
    }

    public void setExecutor(AtCommandExecutor executor) {
        this.executor = executor;
    }

    @Override
    public void open(String pin) throws ModemException {
        try {
            port = SerialPort.getCommPort(portName);
            port.setBaudRate(115200);
            port.setNumDataBits(8);
            port.setNumStopBits(1);
            port.setParity(SerialPort.NO_PARITY);
            port.setFlowControl(SerialPort.FLOW_CONTROL_DISABLED);

            if (!port.openPort()) {
                throw new ModemException("Cannot open port " + portName);
            }

            executor = new AtCommandExecutor(port);

            // basic init
            executor.send("AT", Duration.ofSeconds(2));
            executor.send("ATE0", Duration.ofSeconds(2));
            executor.send("AT+CMGF=1", Duration.ofSeconds(2));

            ensureSimReady(pin);

        } catch (Exception e) {
            throw new ModemException("Failed to initialize modem", e);
        }
    }

    @SuppressWarnings("java:S2925")
    private void ensureSimReady(String pin) throws ModemException {
        logger.info("check PIN");
        // 1. základní handshake
        executor.send("AT", Duration.ofSeconds(2));
        executor.send("ATE0", Duration.ofSeconds(2));

        // 2. zjistit stav SIM
        String cpinResponse = executor.send("AT+CPIN?", Duration.ofSeconds(5));

        if (cpinResponse.contains("READY")) {
            logger.info("SIM already ready.");
        } else if (cpinResponse.contains("SIM PIN")) {

            if (pin == null || pin.isBlank()) {
                throw new IllegalStateException("SIM requires PIN but no PIN provided.");
            }

            System.out.println("Sending SIM PIN...");
            executor.send("AT+CPIN=\"" + pin + "\"", Duration.ofSeconds(5));

            // modem může chvíli nereagovat
            try {
                Thread.sleep(5000);
            } catch (InterruptedException e) {
                throw new ModemException("Cannot wait", e);
            }

            // znovu ověřit
            String verify = executor.send("AT+CPIN?", Duration.ofSeconds(5));
            if (!verify.contains("READY")) {
                throw new IllegalStateException("PIN was not accepted: " + verify);
            }

            logger.info("SIM unlocked.");
        }
        else if (cpinResponse.contains("SIM PUK")) {
            throw new ModemException("SIM is blocked (PUK required).");
        }
        else {
            throw new ModemException("Unknown CPIN response: " + cpinResponse);
        }

        // 3. čekání na registraci do sítě (LTE Cat-M může trvat déle)
        logger.info("Waiting for network registration...");

        long start = System.currentTimeMillis();
        long timeout = 120_000; // 2 min timeout

        while (System.currentTimeMillis() - start < timeout) {

            String reg = executor.send("AT+CEREG?", Duration.ofSeconds(5));

            if (reg.contains(",1") || reg.contains(",5")) {
                System.out.println("Network registered.");
                return;
            }

            try {
                Thread.sleep(5000);
            } catch (InterruptedException e) {
                throw new ModemException("Cannot wait", e);
            }
        }

        throw new ModemException("Network registration timeout.");
    }


    @Override
    public void close() {
        if (port != null && port.isOpen()) {
            port.closePort();
        }
    }

    @Override
    public void sendSms(String number, String message) throws ModemException {
        try {
            executor.sendExpectPrompt("AT+CMGS=\"" + number + "\"", '>', Duration.ofSeconds(2));
            executor.writeMessage(message);
            String response = executor.readUntil("OK", Duration.ofSeconds(10));
            if (!response.contains("+CMGS")) {
                throw new ModemException("SMS not confirmed by modem: " + response);
            }
        } catch (Exception e) {
            throw new ModemException("Failed to send SMS", e);
        }
    }

    @Override
    public List<SmsMessage> readAll() throws ModemException {
        try {
            String response = executor.send("AT+CMGL=\"ALL\"", Duration.ofSeconds(5));
            return parseMessages(response);
        } catch (Exception e) {
            throw new ModemException("Failed to read SMS", e);
        }
    }

    @Override
    public void delete(int index) throws ModemException {
        try {
            executor.send("AT+CMGD=" + index, Duration.ofSeconds(2));
        } catch (Exception e) {
            throw new ModemException("Failed to delete SMS", e);
        }
    }

    private List<SmsMessage> parseMessages(String response) {
        if (response == null || response.isBlank()) {
            return List.of();
        }

        List<String> lines = List.of(response.split("\\r?\\n"));
        return SmsParser.parseCmglResponse(lines);
    }

}
