package kfs.sc.sms.service;

import kfs.sc.sms.model.SmsGateway;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicBoolean;

public class OutgoingSmsService {

    private static final Logger logger = LoggerFactory.getLogger(OutgoingSmsService.class);

    private final SmsGateway gateway;
    private final SmsRestClient client;
    private final long pollIntervalMs;
    private final AtomicBoolean running = new AtomicBoolean(false);
    private ScheduledExecutorService executor;

    public OutgoingSmsService(SmsGateway gateway, SmsRestClient client, long pollIntervalMs) {
        this.gateway = gateway;
        this.client = client;
        this.pollIntervalMs = pollIntervalMs;
    }

    public void start() {
        if (!running.compareAndSet(false, true)) return;

        executor = Executors.newSingleThreadScheduledExecutor(r -> {
            Thread t = new Thread(r, "OutgoingSmsThread");
            t.setDaemon(true);
            return t;
        });

        executor.scheduleAtFixedRate(this::poll, 0, pollIntervalMs, TimeUnit.MILLISECONDS);
        logger.info("OutgoingSmsService started (interval={}ms)", pollIntervalMs);
    }

    private void poll() {
        try {
            SmsRestClient.OutgoingSms sms = client.pollOutgoing();
            if (sms == null || sms.id() == null) {
                return;
            }

            logger.info("Outgoing SMS id={} to={}", sms.id(), sms.numb());

            try {
                gateway.sendSms(sms.numb(), sms.text());
                client.confirmSent(sms.id());
                logger.info("SMS id={} sent OK", sms.id());
            } catch (Exception e) {
                logger.error("SMS id={} send failed: {}", sms.id(), e.getMessage());
                client.reportFail(sms.id(), e.getMessage());
            }
        } catch (Exception e) {
            logger.error("OutgoingSmsService poll error", e);
        }
    }

    public void stop() {
        running.set(false);
        if (executor != null) {
            executor.shutdown();
            try {
                if (!executor.awaitTermination(10, TimeUnit.SECONDS)) {
                    logger.warn("OutgoingSmsService did not terminate in time, forcing shutdown");
                    executor.shutdownNow();
                }
            } catch (InterruptedException e) {
                executor.shutdownNow();
                Thread.currentThread().interrupt();
            }
        }
        logger.info("OutgoingSmsService stopped");
    }
}
