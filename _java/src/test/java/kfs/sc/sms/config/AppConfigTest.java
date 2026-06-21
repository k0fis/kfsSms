package kfs.sc.sms.config;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

import static org.junit.jupiter.api.Assertions.*;

class AppConfigTest {

    @Test
    void shouldLoadValidConfiguration() throws IOException {

        String yaml = """
                sms:
                  portName: "/dev/ttyUSB0"
                  timeout: 1000
                  baudRate: 9600
                  pollIntervalMs: 5000

                api:
                  baseUrl: "https://api.example.com"
                  user: "john"
                  password: "secret"

                logging:
                  level: "DEBUG"
                """;

        Path tempFile = Files.createTempFile("config", ".yml");
        Files.writeString(tempFile, yaml);

        AppConfig config = AppConfig.load(tempFile);

        assertEquals("/dev/ttyUSB0", config.sms().portName());
        assertEquals(9600, config.sms().baudRate());
        assertEquals(5000L, config.sms().pollIntervalMs());

        assertEquals("https://api.example.com", config.api().baseUrl());
        assertEquals("john", config.api().user());
        assertEquals("secret", config.api().password());

        assertEquals("DEBUG", config.logging().level());
    }

    @Test
    void shouldUseDefaultLoggingLevelWhenMissing() throws IOException {

        String yaml = """
                sms:
                  portName: "/dev/ttyUSB0"
                  timeout: 1000
                  baudRate: 9600
                  pollIntervalMs: 5000

                api:
                  baseUrl: "https://api.example.com"
                  user: "john"
                  password: "secret"
                """;

        Path tempFile = Files.createTempFile("config", ".yml");
        Files.writeString(tempFile, yaml);

        AppConfig config = AppConfig.load(tempFile);

        assertEquals("INFO", config.logging().level());
    }

    @Test
    void shouldFailWhenSmsSectionMissing() throws IOException {

        String yaml = """
                api:
                  baseUrl: "https://api.example.com"
                  user: "john"
                  password: "secret"
                """;

        Path tempFile = Files.createTempFile("config", ".yml");
        Files.writeString(tempFile, yaml);

        RuntimeException ex = assertThrows(RuntimeException.class,
                () -> AppConfig.load(tempFile));

        assertTrue(ex.getCause() instanceof IllegalArgumentException);
        assertTrue(ex.getCause().getMessage().contains("Missing config section"));
    }

    @Test
    void shouldFailWhenRequiredValueMissing() throws IOException {

        String yaml = """
                sms:
                  portName: "/dev/ttyUSB0"
                  timeout: 1000
                  baudRate: 9600

                api:
                  baseUrl: "https://api.example.com"
                  user: "john"
                  password: "secret"
                """;

        Path tempFile = Files.createTempFile("config", ".yml");
        Files.writeString(tempFile, yaml);

        RuntimeException ex = assertThrows(RuntimeException.class,
                () -> AppConfig.load(tempFile));

        assertTrue(ex.getCause() instanceof IllegalArgumentException);
        assertTrue(ex.getCause().getMessage().contains("Missing config value"));
    }


    @Test
    void shouldLoadExternalConfigIfExists() throws IOException {
        // připravíme dočasný soubor s jednoduchým YAML
        Path tempFile = Files.createTempFile("test-config", ".yml");
        Files.writeString(tempFile, """
                sms:
                  portName: COM5
                  timeout: 5000
                  baudRate: 115200
                  pollIntervalMs: 100
                api:
                  baseUrl: http://example.com
                  user: test
                  password: secret
                logging:
                  level: DEBUG
                """);

        AppConfig config = AppConfig.loadConfig(tempFile.toString(), "application.yml");

        assertEquals("COM5", config.sms().portName());
        assertEquals("http://example.com", config.api().baseUrl());
        assertEquals("DEBUG", config.logging().level());

        // smažeme temp soubor
        Files.deleteIfExists(tempFile);
    }

    @Test
    void shouldFallbackToResourceIfExternalDoesNotExist() {
        AppConfig config = AppConfig.loadConfig("non-existing-file.yml", "application.yml");

        assertNotNull(config);
        // kontrola, že se načetl resource (např. defaultní port v resources)
        assertNotNull(config.sms().portName());
        assertNotNull(config.api().baseUrl());
        assertNotNull(config.logging().level());
    }

}
