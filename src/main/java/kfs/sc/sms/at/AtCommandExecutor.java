package kfs.sc.sms.at;

import com.fazecast.jSerialComm.SerialPort;
import kfs.sc.sms.utils.ModemException;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.time.Duration;

public class AtCommandExecutor {

    private final SerialPort port;

    public AtCommandExecutor(SerialPort port) {
        this.port = port;
    }

    // Pošli AT příkaz a čekej na OK
    public String send(String command, Duration timeout) throws ModemException {
        writeLine(command);
        return readUntil("OK", timeout);
    }

    // Pošli AT příkaz a čekej na prompt (např. >)
    public String sendExpectPrompt(String command, char prompt, Duration timeout) throws ModemException {
        writeLine(command);
        String response = readUntil(String.valueOf(prompt), timeout);
        if (!response.endsWith(String.valueOf(prompt))) {
            throw new ModemException("Prompt not received: " + response);
        }
        return response;
    }

    // Pošli zprávu (SMS body) + CTRL-Z
    public void writeMessage(String message) throws Exception {
        OutputStream os = port.getOutputStream();
        os.write(message.getBytes());
        os.write(26); // CTRL+Z
        os.flush();
    }

    // Čtení dat až do očekávaného stringu
    @SuppressWarnings("java:S2925")
    public String readUntil(String expected, Duration timeout) throws ModemException {
        InputStream is = port.getInputStream();
        long end = System.currentTimeMillis() + timeout.toMillis();
        StringBuilder sb = new StringBuilder();

        try {
            while (System.currentTimeMillis() < end) {
                while (is.available() > 0) {
                    int b = is.read();
                    if (b == -1) continue;
                    sb.append((char) b);
                    if (sb.toString().contains(expected)) {
                        return sb.toString();
                    }
                }
                Thread.sleep(50);
            }
        } catch ( InterruptedException | IOException e ) {
            throw new ModemException("Cannot read from modem", e);
        }

        throw new ModemException("Timeout waiting for: " + expected + ". Got: " + sb);
    }

    private void writeLine(String line) throws ModemException {
        try {
            OutputStream os = port.getOutputStream();
            os.write((line + "\r").getBytes());
            os.flush();
        } catch (IOException e) {
            throw new ModemException("cannot write line: " + line, e);
        }
    }
}
