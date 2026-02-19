package kfs.sc.sms.at;

import com.fazecast.jSerialComm.SerialPort;
import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.utils.ModemException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.time.Duration;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

class AtModemSmsGatewayTest {

    AtModemSmsGateway gateway;
    AtCommandExecutor executor;
    SerialPort port;
    ByteArrayOutputStream output;
    ByteArrayInputStream input;

    @BeforeEach
    void setup() throws Exception {
        port = mock(SerialPort.class);
        output = new ByteArrayOutputStream();
        when(port.getOutputStream()).thenReturn(output);

        executor = mock(AtCommandExecutor.class);
        gateway = new AtModemSmsGateway("COM1");
        gateway.setExecutor(executor);
    }

    @Test
    void shouldSendSmsAndConfirm() throws Exception {
        when(executor.sendExpectPrompt(anyString(), anyChar(), any())).thenReturn(null);
        when(executor.readUntil("OK", Duration.ofSeconds(10))).thenReturn("+CMGS: 1\r\nOK\r\n");

        gateway.sendSms("+420123456789", "Hello World");

        verify(executor).sendExpectPrompt("AT+CMGS=\"+420123456789\"", '>', Duration.ofSeconds(2));
        verify(executor).writeMessage("Hello World");
        verify(executor).readUntil("OK", Duration.ofSeconds(10));
    }

    @Test
    void shouldThrowIfSmsNotConfirmed() throws Exception {
        when(executor.sendExpectPrompt(anyString(), anyChar(), any())).thenReturn(null);
        when(executor.readUntil("OK", Duration.ofSeconds(10))).thenReturn("ERROR\r\n");

        ModemException e = assertThrows(ModemException.class,
                () -> gateway.sendSms("+420123456789", "Hello"));

        assertTrue(e.getMessage().contains("Failed to send SMS"));
    }

    @Test
    void shouldParseMessagesFromResponse() throws Exception {
        String response = "+CMGL: 1,\"REC READ\",\"+420111111111\",\"\",\"24/02/12,09:41:22+04\"\r\nHello\r\nOK";
        when(executor.send(anyString(), any())).thenReturn(response);

        List<SmsMessage> messages = gateway.readAll();

        assertEquals(1, messages.size());
        assertEquals("Hello", messages.get(0).getBody());
    }

    @Test
    void shouldDeleteMessage() throws Exception {
        gateway.delete(5);
        verify(executor).send("AT+CMGD=5", Duration.ofSeconds(2));
    }

    @Test
    void shouldParseMultiLineSms() throws Exception {
        String response =
                "+CMGL: 1,\"REC READ\",\"+420111111111\",\"\",\"24/02/12,09:41:22+04\"\r\n" +
                        "Hello line 1\r\n" +
                        "Hello line 2\r\n" +
                        "OK";

        when(executor.send(anyString(), any())).thenReturn(response);

        List<SmsMessage> messages = gateway.readAll();

        assertEquals(1, messages.size());
        assertEquals("Hello line 1\nHello line 2", messages.get(0).getBody());
    }

    @Test
    void shouldReturnEmptyListOnErrorResponse() throws Exception {
        when(executor.send(anyString(), any())).thenReturn("ERROR");

        List<SmsMessage> messages = gateway.readAll();
        assertTrue(messages.isEmpty());
    }

    @Test
    void shouldReturnEmptyListOnCmsErrorResponse() throws Exception {
        when(executor.send(anyString(), any())).thenReturn("+CMS ERROR: 500");

        List<SmsMessage> messages = gateway.readAll();
        assertTrue(messages.isEmpty());
    }
    @Test
    void pollingLoopShouldNotFreezeOnTimeout() throws Exception {
        when(executor.send(anyString(), any())).thenThrow(new ModemException("Timeout"));

        // simulace polling loop jednou
        Runnable polling = () -> {
            try {
                gateway.readAll();
            } catch (ModemException ignored) {
            }
        };

        Thread thread = new Thread(polling);
        thread.start();
        thread.join(1000); // test timeout: thread nesmí viset

        assertFalse(thread.isAlive(), "Polling loop should not freeze on timeout");
    }

}
