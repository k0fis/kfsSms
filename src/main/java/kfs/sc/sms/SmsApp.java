package kfs.sc.sms;

import kfs.sc.sms.at.AtModemSmsGateway;
import kfs.sc.sms.config.AppConfig;
import kfs.sc.sms.model.SmsGateway;
import kfs.sc.sms.model.SmsMessage;
import kfs.sc.sms.service.SmsDispatchService;
import kfs.sc.sms.service.SmsPollingService;
import kfs.sc.sms.service.SmsRestClient;
import kfs.sc.sms.updater.GitHubUpdater;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.time.Duration;
import java.time.temporal.ChronoUnit;
import java.util.Map;
import java.util.concurrent.ArrayBlockingQueue;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.CountDownLatch;

public class SmsApp {

    private static final Logger logger = LoggerFactory.getLogger(SmsApp.class);

    public static void main(String[] args) throws InterruptedException {
        String externalPath = args.length > 0 ? args[0] : null;

        logger.info("Start SmsApp with config: '{}'", externalPath);
        AppConfig config = AppConfig.loadConfig(externalPath, "application.yml");

        // Init logging
        initLogger(config.logging().level());
        initLoggerPackages(config.logging().packages());

        // check updates
        GitHubUpdater updater = new GitHubUpdater(
                "k0fis",
                "kfsSms",
                "SmsApp.jar",
                getVersion()
        );

        if (updater.updateIfAvailable()) {
            logger.info("Updated. Restarting...");
            updater.restartApplication();
        }

        // SMS Gateway
        SmsGateway smsGateway = new AtModemSmsGateway(config.sms().portName());
        if (config.sms().openModem()) {
            try {
                smsGateway.open(config.getMsisdn().pin());
            } catch (Exception ex) {
                logger.error("Error initialize modem", ex);
                System.exit(-1);
            }
        }

        // REST client
        SmsRestClient smsRestClient = new SmsRestClient(
                config.api().baseUrl(),
                config.api().user(),
                config.api().password()
        );

        // Fronta pro SMS
        BlockingQueue<SmsMessage> queue = new ArrayBlockingQueue<>(1000);

        // Polling service (modem → fronta)
        SmsPollingService pollingService = new SmsPollingService(
                smsGateway,
                Duration.of(config.sms().pollIntervalMs(), ChronoUnit.MILLIS),
                queue, config.sms().poolRetryCount(), config.sms().sendRetryDelayMs()
        );

        // Dispatch service (fronta → REST)
        SmsDispatchService dispatchService = new SmsDispatchService(
                queue,
                smsRestClient,
                config.sms().sendMaxRetries(),
                config.sms().sendRetryDelayMs()
        );

        // Spuštění obou služeb
        pollingService.start();
        dispatchService.start();

        CountDownLatch shutdownLatch = new CountDownLatch(1);

        // JVM shutdown hook
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            logger.info("Shutdown signal received");

            try {
                pollingService.stop();
                dispatchService.stop();
                smsGateway.close();
            } catch (Exception e) {
                logger.error("Error during shutdown", e);
            }

            shutdownLatch.countDown();
            logger.info("SmsApp stopped");
        }));

        if (config.cfg().terminate() != null && !config.cfg().terminate().isEmpty()) {
            new Thread(() -> {
                try (java.util.Scanner scanner = new java.util.Scanner(System.in)) {
                    logger.info("Type '{}' + Enter to stop application", config.cfg().terminate());
                    while (true) {
                        String line = scanner.nextLine();
                        if (config.cfg().terminate().equalsIgnoreCase(line.trim())) {
                            logger.info("Manual shutdown requested");
                            pollingService.stop();
                            dispatchService.stop();
                            smsGateway.close();
                            shutdownLatch.countDown();
                            break;
                        }
                    }
                } catch (Exception e) {
                    logger.debug("Console listener stopped", e);
                }
            }, "console-listener").start();
        }

        logger.info("SmsApp started, waiting for shutdown signal");
        shutdownLatch.await();
    }

    private static void initLogger(String levelName) {
        ch.qos.logback.classic.Logger rootLogger =
                (ch.qos.logback.classic.Logger) LoggerFactory.getLogger(org.slf4j.Logger.ROOT_LOGGER_NAME);
        setLevel(rootLogger, levelName);
        rootLogger.info("Root log level set to {}", rootLogger.getLevel());
    }

    private static void initLoggerPackages(Map<String, String> packages) {
        if (packages == null) return;

        for (Map.Entry<String, String> entry : packages.entrySet()) {
            ch.qos.logback.classic.Logger logger =
                    (ch.qos.logback.classic.Logger) LoggerFactory.getLogger(entry.getKey());
            setLevel(logger, entry.getValue());
            logger.info("Log level for '{}' set to {}", entry.getKey(), logger.getLevel());
        }
    }

    private static void setLevel(ch.qos.logback.classic.Logger logger, String level) {
        try {
            logger.setLevel(ch.qos.logback.classic.Level.valueOf(level.toUpperCase()));
        } catch (IllegalArgumentException e) {
            logger.warn("Invalid log level '{}', keeping default", level);
        }
    }

    public static String getVersion() {
        return SmsApp.class.getPackage().getImplementationVersion();
    }
}
