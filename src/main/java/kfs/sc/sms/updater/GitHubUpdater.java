package kfs.sc.sms.updater;

import kfs.sc.sms.utils.KfsSmsException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.*;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class GitHubUpdater {

    private static final Logger logger = LoggerFactory.getLogger(GitHubUpdater.class);

    private final String owner;
    private final String repo;
    private final String localJarName;
    private final String currentVersion;

    private final HttpClient client = HttpClient.newHttpClient();

    public GitHubUpdater(String owner, String repo, String localJarName, String currentVersion) {
        this.owner = owner;
        this.repo = repo;
        this.localJarName = localJarName;
        this.currentVersion = currentVersion;
    }

    public boolean updateIfAvailable() {
        logger.info("Checking for updates...");

        ReleaseInfo latest = fetchLatestRelease();

        if (latest == null) {
            logger.info("Unable to fetch latest release.");
            return false;
        }

        if (!isNewerVersion(currentVersion, latest.version)) {
            logger.info("Already up to date.");
            return false;
        }

        logger.info("New version available: " + latest.version);

        Path downloaded = downloadJar(latest.downloadUrl);
        backupCurrentJar();
        replaceJar(downloaded);

        logger.info("Update complete.");
        return true;
    }

    public void restartApplication()  {
        try {
            new ProcessBuilder("java", "-jar", localJarName)
                    .inheritIO()
                    .start();
        } catch (IOException e) {
            throw new KfsSmsException("Cannot run new process", e);
        }

        System.exit(0);
    }

    private ReleaseInfo fetchLatestRelease()  {
        String url = "https://api.github.com/repos/" + owner + "/" + repo + "/releases/latest";

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Accept", "application/vnd.github+json")
                .header("User-Agent", "kfsSms-updater")
                .GET()
                .build();

        HttpResponse<String> response;
        try {
            response = client.send(request, HttpResponse.BodyHandlers.ofString());
        } catch (InterruptedException | IOException e) {
            throw new KfsSmsException("Cannot fetch last release", e);
        }

        if (response.statusCode() != 200) {
            return null;
        }

        String body = response.body();

        String tag = extract(body, "\"tag_name\"\\s*:\\s*\"([^\"]+)\"");
        if (tag == null) return null;

        String version = tag.startsWith("v") ? tag.substring(1) : tag;

        String jarPattern = "\"browser_download_url\"\\s*:\\s*\"([^\"]+SmsApp-" + version + "\\.jar)\"";
        String downloadUrl = extract(body, jarPattern);

        if (downloadUrl == null) return null;

        return new ReleaseInfo(version, downloadUrl);
    }

    private Path downloadJar(String url) {
        logger.info("Downloading: " + url);

        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("User-Agent", "kfsSms-updater")
                .GET()
                .build();

        Path tempFile = Paths.get("SmsApp-new.jar");

        try {
            client.send(request, HttpResponse.BodyHandlers.ofFile(tempFile));
        } catch (IOException | InterruptedException e) {
            throw new KfsSmsException("Cannot download jar", e);
        }

        return tempFile;
    }

    private void backupCurrentJar() {
        Path current = Paths.get(localJarName);
        if (Files.exists(current)) {
            Path backup = Paths.get(localJarName + ".backup");
            if (Files.exists(backup)) {
                try {
                    Files.delete(backup);
                } catch (IOException e) {
                    throw new KfsSmsException("Cannot delete old backup", e);
                }
            }
            try {
                Files.copy(current, backup, StandardCopyOption.REPLACE_EXISTING);
            } catch (IOException e) {
                throw new KfsSmsException("Cannot create backup of jar", e);
            }
        }
    }

    private void replaceJar(Path newJar) {
        try {
            Files.move(newJar,
                    Paths.get(localJarName),
                    StandardCopyOption.REPLACE_EXISTING,
                    StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException e) {
            throw new KfsSmsException("Cannot move jar", e);
        }
    }

    private boolean isNewerVersion(String current, String latest) {
        return compareVersions(latest, current) > 0;
    }

    private int compareVersions(String v1, String v2) {
        String[] a1 = v1.split("\\.");
        String[] a2 = v2.split("\\.");

        int len = Math.max(a1.length, a2.length);

        for (int i = 0; i < len; i++) {
            int n1 = i < a1.length ? Integer.parseInt(a1[i]) : 0;
            int n2 = i < a2.length ? Integer.parseInt(a2[i]) : 0;

            if (n1 != n2) {
                return Integer.compare(n1, n2);
            }
        }
        return 0;
    }

    private String extract(String text, String regex) {
        Pattern pattern = Pattern.compile(regex);
        Matcher matcher = pattern.matcher(text);
        return matcher.find() ? matcher.group(1) : null;
    }

    private static class ReleaseInfo {
        String version;
        String downloadUrl;

        ReleaseInfo(String version, String downloadUrl) {
            this.version = version;
            this.downloadUrl = downloadUrl;
        }
    }
}