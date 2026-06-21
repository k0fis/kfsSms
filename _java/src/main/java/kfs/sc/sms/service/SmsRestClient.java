package kfs.sc.sms.service;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.OffsetDateTime;
import java.time.format.DateTimeFormatter;
import java.util.Base64;

public class SmsRestClient {

    private static final Logger logger = LoggerFactory.getLogger(SmsRestClient.class);

    private final String baseUrl;
    private final String authHeader;
    private final HttpClient client;

    public SmsRestClient(String baseUrl, String user, String password) {
        this.baseUrl = baseUrl;
        this.authHeader = "Basic " + Base64.getEncoder()
                .encodeToString((user + ":" + password).getBytes(StandardCharsets.UTF_8));
        this.client = HttpClient.newHttpClient();
    }

    public record OutgoingSms(String id, String numb, String text) {}

    /**
     * GET /sms/o → parse JSON {id, numb, text}, returns null if nothing pending.
     */
    public OutgoingSms pollOutgoing() {
        try {
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(baseUrl + "/sms/o"))
                    .header("Authorization", authHeader)
                    .GET()
                    .build();

            HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() != 200) {
                logger.warn("pollOutgoing: HTTP {}", response.statusCode());
                return null;
            }

            String body = response.body();
            if (body == null || body.isBlank() || body.trim().equals("{}")) {
                return null;
            }

            String id = extractJson(body, "id");
            String numb = extractJson(body, "numb");
            String text = extractJson(body, "text");

            if (id == null || id.isEmpty()) {
                return null;
            }

            return new OutgoingSms(id, numb, text);
        } catch (IOException | InterruptedException e) {
            logger.error("pollOutgoing failed", e);
            return null;
        }
    }

    /**
     * POST /sms/i — report incoming SMS from modem to server.
     */
    public void reportIncoming(String numb, String text, OffsetDateTime time) {
        String json = "{\"numb\":\"" + escapeJson(numb)
                + "\",\"text\":\"" + escapeJson(text)
                + "\",\"time\":\"" + time.format(DateTimeFormatter.ISO_OFFSET_DATE_TIME) + "\"}";
        post("/sms/i", json);
    }

    /**
     * POST /sms/c — confirm SMS was sent by modem.
     */
    public void confirmSent(String id) {
        String json = "{\"id\":\"" + escapeJson(id)
                + "\",\"time\":\"" + OffsetDateTime.now().format(DateTimeFormatter.ISO_OFFSET_DATE_TIME) + "\"}";
        post("/sms/c", json);
    }

    /**
     * POST /sms/f — report SMS send failure.
     */
    public void reportFail(String id, String mesg) {
        String json = "{\"id\":\"" + escapeJson(id)
                + "\",\"time\":\"" + OffsetDateTime.now().format(DateTimeFormatter.ISO_OFFSET_DATE_TIME)
                + "\",\"mesg\":\"" + escapeJson(mesg) + "\"}";
        post("/sms/f", json);
    }

    /**
     * POST /log — send plain text log to server.
     */
    public void sendLog(String text) {
        post("/log", text);
    }

    private void post(String path, String data) {
        try {
            String encoded = "data=" + URLEncoder.encode(data, StandardCharsets.UTF_8);

            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(baseUrl + path))
                    .header("Authorization", authHeader)
                    .header("Content-Type", "application/x-www-form-urlencoded")
                    .POST(HttpRequest.BodyPublishers.ofString(encoded))
                    .build();

            HttpResponse<String> response = client.send(request, HttpResponse.BodyHandlers.ofString());

            if (response.statusCode() >= 400) {
                logger.warn("POST {} returned HTTP {}: {}", path, response.statusCode(), response.body());
            } else {
                logger.debug("POST {} OK ({})", path, response.statusCode());
            }
        } catch (IOException | InterruptedException e) {
            logger.error("POST {} failed", path, e);
        }
    }

    private static String extractJson(String json, String key) {
        String pattern = "\"" + key + "\"\\s*:\\s*\"([^\"]*?)\"";
        var matcher = java.util.regex.Pattern.compile(pattern).matcher(json);
        return matcher.find() ? matcher.group(1) : null;
    }

    private static String escapeJson(String s) {
        if (s == null) return "";
        return s.replace("\\", "\\\\")
                .replace("\"", "\\\"")
                .replace("\n", "\\n")
                .replace("\r", "\\r")
                .replace("\t", "\\t");
    }
}
