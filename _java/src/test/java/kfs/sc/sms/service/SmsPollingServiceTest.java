package kfs.sc.sms.service;

import kfs.sc.sms.model.SmsGateway;
import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.utils.ModemException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.BlockingQueue;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.Mockito.*;

class SmsPollingServiceTest {

    SmsGateway gateway;
    BlockingQueue<SmsMessage> queue;
    SmsPollingService pollingService;

    @BeforeEach
    void setup() throws ModemException {
        gateway = mock(SmsGateway.class);
        queue = new ArrayBlockingQueue<>(10);

        // otevření/uzavření mocku
        doNothing().when(gateway).open("pin");
        doNothing().when(gateway).close();
        doNothing().when(gateway).delete(anyInt());
    }

    @Test
    void shouldProcessSingleSms() throws Exception {
        SmsMessage msg = new SmsMessage(1, null, "+420111111111", null, "Hello");
        when(gateway.readAll()).thenReturn(List.of(msg));

        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), queue, 1, 50);
        pollingService.start();

        Thread.sleep(200);
        pollingService.stop();

        SmsMessage taken = queue.poll();
        assertNotNull(taken);
        assertEquals(msg.getIndex(), taken.getIndex());
        verify(gateway, atLeastOnce()).delete(msg.getIndex());
    }

    @Test
    void shouldHandleEmptyListGracefully() throws Exception {
        when(gateway.readAll()).thenReturn(List.of());

        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), queue, 1, 50);
        pollingService.start();

        Thread.sleep(150);
        pollingService.stop();

        assertTrue(queue.isEmpty());
        verify(gateway, never()).delete(anyInt());
    }

    @Test
    void shouldHandleModemException() throws Exception {
        when(gateway.readAll()).thenThrow(new ModemException("timeout"));

        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), queue, 1, 50);
        pollingService.start();

        Thread.sleep(150);
        pollingService.stop();

        assertTrue(queue.isEmpty());
        verify(gateway, never()).delete(anyInt());
    }

    @Test
    void shouldStopPollingLoop() throws Exception {
        when(gateway.readAll()).thenReturn(List.of());
        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), queue, 1, 50);

        pollingService.start();
        assertTrue(pollingService.isRunning());

        pollingService.stop();
        assertFalse(pollingService.isRunning());
    }

    @Test
    void shouldProcessMultipleSms() throws Exception {
        SmsMessage msg1 = new SmsMessage(1, null, "+420111111111", null, "Hello1");
        SmsMessage msg2 = new SmsMessage(2, null, "+420222222222", null, "Hello2");

        when(gateway.readAll()).thenReturn(List.of(msg1, msg2)).thenReturn(List.of());

        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), queue, 1, 50);
        pollingService.start();

        Thread.sleep(200);
        pollingService.stop();

        assertEquals(2, queue.size());
        assertTrue(queue.stream().anyMatch(m -> m.getIndex() == 1));
        assertTrue(queue.stream().anyMatch(m -> m.getIndex() == 2));
        verify(gateway).delete(1);
        verify(gateway).delete(2);
    }

    @Test
    void shouldRetryAndFailGracefully() throws Exception {
        SmsMessage msg = new SmsMessage(1, null, "+420111111111", null, "Hello");

        // první 2 pokusy vyvolají exception při queue.put
        BlockingQueue<SmsMessage> failingQueue = spy(new ArrayBlockingQueue<>(1));
        doThrow(new RuntimeException("full")).doThrow(new RuntimeException("full")).doCallRealMethod()
                .when(failingQueue).put(any());

        when(gateway.readAll()).thenReturn(List.of(msg));

        pollingService = new SmsPollingService(gateway, Duration.ofMillis(50), failingQueue, 2, 10);
        pollingService.start();

        Thread.sleep(200);
        pollingService.stop();

        // nakonec se pokusí vložit a selže maxRetries, fronta může být prázdná
        verify(failingQueue, atLeast(3)).put(msg);
        verify(gateway, atLeast(1)).delete(msg.getIndex());
    }
}
