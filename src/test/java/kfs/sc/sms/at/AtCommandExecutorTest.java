package kfs.sc.sms.at;

import com.fazecast.jSerialComm.SerialPort;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.time.Duration;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

class AtCommandExecutorTest {

    SerialPort port;
    ByteArrayOutputStream output;
    ByteArrayInputStream input;
    AtCommandExecutor executor;

    @BeforeEach
    void setup() {
        port = mock(SerialPort.class);
        output = new ByteArrayOutputStream();

        when(port.getOutputStream()).thenReturn(output);

        // input bude naplněn inline v testu
    }

    @Test
    void shouldSendCommandAndReceiveOk() throws Exception {
        String modemResponse = "AT\r\nOK\r\n";
        input = new ByteArrayInputStream(modemResponse.getBytes());
        when(port.getInputStream()).thenReturn(input);

        executor = new AtCommandExecutor(port);

        String response = executor.send("AT", Duration.ofSeconds(2));
        assertTrue(response.contains("OK"));
        assertTrue(output.toString().contains("AT\r"));
    }

    @Test
    void shouldThrowOnTimeout() {
        input = new ByteArrayInputStream(new byte[0]);
        when(port.getInputStream()).thenReturn(input);

        executor = new AtCommandExecutor(port);

        Exception e = assertThrows(Exception.class, () ->
                executor.send("AT", Duration.ofMillis(50))
        );

        assertTrue(e.getMessage().contains("Timeout"));
    }

    @Test
    void shouldSendMessageWithCtrlZ() throws Exception {
        input = new ByteArrayInputStream("OK".getBytes());
        when(port.getInputStream()).thenReturn(input);

        executor = new AtCommandExecutor(port);
        executor.writeMessage("Hello");

        byte[] bytes = output.toByteArray();
        assertTrue(new String(bytes).contains("Hello"));
        assertEquals(26, bytes[bytes.length - 1]); // CTRL-Z
    }
}
