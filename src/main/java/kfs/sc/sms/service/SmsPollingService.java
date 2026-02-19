package kfs.sc.sms.service;

import kfs.sc.sms.model.SmsGateway;
import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.utils.ModemException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.util.List;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

public class SmsPollingService {

    private static final Logger logger = LoggerFactory.getLogger(SmsPollingService.class);

    private final SmsGateway gateway;
    private final Duration interval;
    private final BlockingQueue<SmsMessage> queue;
    private final int maxRetries;
    private final long retryDelayMs;
    private final AtomicBoolean running = new AtomicBoolean(false);
    private ScheduledExecutorService executor;

    public SmsPollingService(SmsGateway gateway,
                             Duration interval,
                             BlockingQueue<SmsMessage> queue,
                             int maxRetries,
                             long retryDelayMs) {
        this.gateway = gateway;
        this.interval = interval;
        this.queue = queue;
        this.maxRetries = maxRetries;
        this.retryDelayMs = retryDelayMs;
    }

    public void start() {
        if (!running.compareAndSet(false, true)) return;

        executor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "SmsPollingThread");
            t.setDaemon(true);
            return t;
        });

        executor.scheduleAtFixedRate(this::poll, 0, interval.toMillis(), TimeUnit.MILLISECONDS);
        logger.info("SmsPollingService started");
    }

    private void poll() {
        try {
            List<SmsMessage> messages = gateway.readAll();
            for (SmsMessage msg : messages) {
                boolean success = false;
                int attempt = 0;

                while (!success && attempt <= maxRetries) {
                    attempt++;
                    try {
                        queue.put(msg); // přidej do fronty pro dispatch
                        gateway.delete(msg.getIndex()); // smaž po úspěchu
                        success = true;
                    } catch (Exception e) {
                        logger.warn("Failed to enqueue SMS (attempt {}): {}", attempt, e.getMessage());
                        if (attempt <= maxRetries) {
                            Thread.sleep(retryDelayMs);
                        }
                    }
                }

                if (!success) {
                    logger.error("Failed to process SMS after {} attempts: {}", maxRetries, msg);
                }
            }
        } catch (ModemException e) {
            logger.error("Error reading SMS", e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }

    public void stop() {
        running.set(false);

        if (executor != null) {
            executor.shutdown();
            try {
                if (!executor.awaitTermination(10, TimeUnit.SECONDS)) {
                    logger.warn("Polling did not terminate in time, forcing shutdown");
                    executor.shutdownNow();
                }
            } catch (InterruptedException e) {
                executor.shutdownNow();
                Thread.currentThread().interrupt();
            }
        }

        // Graceful drain: fronta se nevyprázdní zde, zařídí to dispatch service
        logger.info("SmsPollingService stopped");
    }

    public boolean isRunning() {
        return running.get();
    }
}

