package kfs.sc.sms.config;

import kfs.sc.sms.utils.KfsSmsException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.yaml.snakeyaml.Yaml;

import java.io.InputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Map;

public final class AppConfig {

    private static final Logger logger = LoggerFactory.getLogger(AppConfig.class);

    private final CfgConfig cfg;
    private final SmsConfig sms;
    private final ApiConfig api;
    private final MsisdnConfig msisdn;
    private final LoggingConfig logging;

    private AppConfig(SmsConfig sms, ApiConfig api, LoggingConfig logging, CfgConfig cfg, MsisdnConfig msisdn) {
        this.sms = sms;
        this.api = api;
        this.logging = logging;
        this.cfg = cfg;
        this.msisdn = msisdn;
    }

    public static AppConfig loadConfig(String paramPath, String fallbackResource) {
        if (paramPath != null) {
            Path path = Path.of(paramPath);
            if (Files.exists(path)) {
                return AppConfig.load(path);
            } else {
                logger.warn("Warning: external config file not found: " + paramPath
                        + " → loading default from resources");
            }
        }
        return AppConfig.loadFromClasspath(fallbackResource);
    }

    private static AppConfig load(InputStream is) {
        Yaml yaml = new Yaml();
        Map<String, Object> root = yaml.load(is);
        SmsConfig sms = SmsConfig.from((Map<String, Object>) root.get("sms"));
        ApiConfig api = ApiConfig.from((Map<String, Object>) root.get("api"));
        LoggingConfig logging = LoggingConfig.from((Map<String, Object>) root.get("logging"));
        CfgConfig cfg = CfgConfig.from((Map<String, Object>) root.get("cfg"));
        MsisdnConfig msisdn = MsisdnConfig.from((Map<String, Object>) root.get("msisdn"));

        return new AppConfig(sms, api, logging, cfg, msisdn);
    }

    public static AppConfig load(Path path) {
        try (InputStream is = Files.newInputStream(path)) {
            return load(is);
        } catch (Exception e) {
            throw new KfsSmsException("Failed to load configuration from " + path, e);
        }
    }

    public static AppConfig loadFromClasspath(String resourceName) {
        try (InputStream is = AppConfig.class
                .getClassLoader()
                .getResourceAsStream(resourceName)) {

            if (is == null) {
                throw new IllegalArgumentException("Resource not found: " + resourceName);
            }
            return load(is);
        } catch (Exception e) {
            throw new KfsSmsException("Failed to load configuration from classpath: " + resourceName, e);
        }
    }


    public SmsConfig sms() {
        return sms;
    }

    public ApiConfig api() {
        return api;
    }

    public LoggingConfig logging() {
        return logging;
    }

    public CfgConfig cfg() {
        return cfg;
    }

    public MsisdnConfig getMsisdn() {
        return msisdn;
    }

    // ==========================
    // Nested config classes
    // ==========================

    public record SmsConfig(
            String portName,
            int baudRate,
            long pollIntervalMs,
            boolean openModem,
            int sendMaxRetries,
            long sendRetryDelayMs,
            int poolRetryCount,
            long poolRetryDelay
            ) {

        static SmsConfig from(Map<String, Object> map) {
            require(map, "sms");

            return new SmsConfig(
                    requireString(map, "portName"),
                    requireInt(map, "baudRate"),
                    requireLong(map, "pollIntervalMs"),
                    requireBoolean(map, "openModem", true),
                    requireInt(map, "sendMaxRetries", 3),
                    requireLong(map, "sendRetryDelayMs", 1000),
                    requireInt(map, "poolRetryCount", 3),
                    requireLong(map, "poolRetryDelay", 5000)
            );
        }
    }

    public record ApiConfig(String baseUrl, String user, String password) {

        static ApiConfig from(Map<String, Object> map) {
            require(map, "api");

            return new ApiConfig(
                    requireString(map, "baseUrl"),
                    requireString(map, "user"),
                    requireString(map, "password")
            );
        }
    }

    public record LoggingConfig(
            String level,
            Map<String, String> packages
    ) {

        static LoggingConfig from(Map<String, Object> map) {
            if (map == null) {
                return new LoggingConfig("INFO", Map.of());
            }
            String level = map.getOrDefault("level", "INFO").toString();
            Map<String, String> packages = Map.of();
            Object pkgObj = map.get("packages");
            if (pkgObj instanceof Map<?, ?> pkgMap) {
                packages = pkgMap.entrySet().stream()
                        .collect(java.util.stream.Collectors.toMap(
                                e -> e.getKey().toString(),
                                e -> e.getValue().toString()
                        ));
            }
            return new LoggingConfig(level, packages);
        }
    }

    public record CfgConfig(String terminate) {
        static CfgConfig from(Map<String, Object> map) {
            return new CfgConfig(requireString(map, "terminate", ""));
        }
    }

    public record MsisdnConfig(String pin) {
        static MsisdnConfig from(Map<String, Object> map) {
            return new MsisdnConfig(requireString(map, "pin", ""));
        }
    }

    // ==========================
    // Validation helpers
    // ==========================

    private static void require(Map<String, Object> map, String section) {
        if (map == null) {
            throw new IllegalArgumentException("Missing config section: " + section);
        }
    }

    private static String requireString(Map<String, Object> map, String key) {
        Object val = map.get(key);
        if (val == null || val.toString().isBlank()) {
            throw new IllegalArgumentException("Missing config value: " + key);
        }
        return val.toString();
    }

    private static String requireString(Map<String, Object> map, String key, String defaultValue) {
        if (map == null) {
            return defaultValue;
        }
        Object val = map.get(key);
        if (val == null || val.toString().isBlank()) {
            return defaultValue;
        }
        return val.toString();
    }


    private static int requireInt(Map<String, Object> map, String key) {
        Object val = map.get(key);
        if (val == null) {
            throw new IllegalArgumentException("Missing config value: " + key);
        }
        return Integer.parseInt(val.toString());
    }

    private static int requireInt(Map<String, Object> map, String key, int defaultValue) {
        Object val = map.get(key);
        if (val == null) {
            return defaultValue;
        }
        return Integer.parseInt(val.toString());
    }

    private static long requireLong(Map<String, Object> map, String key) {
        Object val = map.get(key);
        if (val == null) {
            throw new IllegalArgumentException("Missing config value: " + key);
        }
        return Long.parseLong(val.toString());
    }

    private static long requireLong(Map<String, Object> map, String key, long defaultValue) {
        Object val = map.get(key);
        if (val == null) {
            return defaultValue;
        }
        return Long.parseLong(val.toString());
    }

    private static boolean requireBoolean(Map<String, Object> map, String key, boolean defaultValue) {
        Object val = map.get(key);

        if (val == null) {
            return defaultValue;
        }

        if (val instanceof Boolean b) {
            return b;
        }

        String str = val.toString().trim().toLowerCase();

        if ("true".equals(str)) return true;
        if ("false".equals(str)) return false;

        logger.warn("Invalid boolean value '{}' for key '{}', using default={}", val, key, defaultValue);

        return defaultValue;
    }

}

