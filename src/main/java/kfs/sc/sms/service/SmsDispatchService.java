package kfs.sc.sms.service;

import kfs.sc.sms.model.SmsMessage;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

public class SmsDispatchService {

    private static final Logger logger = LoggerFactory.getLogger(SmsDispatchService.class);

    private final BlockingQueue<SmsMessage> queue;
    private final SmsRestClient client;
    private final ExecutorService executor;
    private final int maxRetries;
    private final long retryDelayMs;
    private final AtomicBoolean running = new AtomicBoolean(false);

    public SmsDispatchService(BlockingQueue<SmsMessage> queue,
                              SmsRestClient client,
                              int maxRetries,
                              long retryDelayMs) {
        this.queue = queue;
        this.client = client;
        this.maxRetries = maxRetries;
        this.retryDelayMs = retryDelayMs;

        this.executor = Executors.newSingleThreadExecutor(r -> {
            Thread t = new Thread(r, "SmsDispatchThread");
            t.setDaemon(true);
            return t;
        });
    }

    public void start() {
        if (!running.compareAndSet(false, true)) return;

        executor.submit(() -> {
            while (running.get() || !queue.isEmpty()) {
                try {
                    SmsMessage msg = queue.poll(1, TimeUnit.SECONDS);
                    if (msg != null) {
                        processWithRetry(msg);
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    break;
                }
            }
            logger.info("SmsDispatchService stopped, queue drained");
        });

        logger.info("SmsDispatchService started");
    }

    private void processWithRetry(SmsMessage msg) {
        int attempt = 0;
        while (attempt <= maxRetries) {
            attempt++;
            try {
                logger.debug("SMS try to send ({}): {}", attempt, msg);
                client.send(msg);
                logger.info("SMS sent successfully: {}", msg);
                return;
            } catch (Exception e) {
                logger.warn("Attempt {} failed to send SMS: {} - {}", attempt, msg, e.getMessage());
                if (attempt > maxRetries) {
                    logger.error("Failed to deliver SMS after {} retries: {}", maxRetries, msg);
                    return;
                }
                try {
                    Thread.sleep(retryDelayMs);
                } catch (InterruptedException ex) {
                    Thread.currentThread().interrupt();
                    return;
                }
            }
        }
    }

    public void stop() {
        running.set(false);
        executor.shutdown();
        try {
            if (!executor.awaitTermination(30, TimeUnit.SECONDS)) {
                logger.warn("Dispatch thread did not terminate in time, forcing shutdown");
                executor.shutdownNow();
            }
        } catch (InterruptedException e) {
            executor.shutdownNow();
            Thread.currentThread().interrupt();
        }
    }
}
