# kfsSms — Claude Code projekt notes

## Build & Run

```bash
mvn clean package          # build fat JAR + testy
mvn test                   # jen testy

# lokalni test (modem vypnuty)
java -jar target/SmsApp-0.0.1.jar src/main/resources/application.yml
```

## Struktura

```
kfsSms/
├── src/main/java/kfs/sc/sms/
│   ├── SmsApp.java                    # main, wiring, shutdown hooks
│   ├── at/
│   │   ├── AtModemSmsGateway.java     # SmsGateway impl (jSerialComm, AT prikazy)
│   │   ├── AtCommandExecutor.java     # AT command I/O pres serial port
│   │   └── SmsParser.java            # parsovani CMGL response
│   ├── config/
│   │   └── AppConfig.java            # YAML config (SnakeYAML), nested records
│   ├── model/
│   │   ├── SmsGateway.java           # interface: open/close/sendSms/readAll/delete
│   │   ├── SmsMessage.java           # record(index, status, sender, timestamp, text)
│   │   ├── SmsStatus.java            # enum
│   │   └── ModemSession.java / ModemState.java
│   ├── service/
│   │   ├── SmsPollingService.java    # modem → queue (ScheduledExecutor)
│   │   ├── SmsDispatchService.java   # queue → REST (reportIncoming)
│   │   ├── OutgoingSmsService.java   # REST → modem (pollOutgoing → sendSms → confirm/fail)
│   │   └── SmsRestClient.java        # java.net.http.HttpClient, Basic Auth, form-urlencoded
│   ├── updater/
│   │   └── GitHubUpdater.java        # GitHub Releases check, download SmsApp-new.jar
│   └── utils/
│       ├── KfsSmsException.java
│       └── ModemException.java
├── src/main/resources/application.yml
├── kfsSms.bat                         # Windows wrapper (exit 42 = update swap)
├── install.ps1                        # PowerShell installer (JRE + JAR + Task Scheduler)
├── .github/workflows/ci.yml          # GitHub Actions: build + release JAR
└── pom.xml                            # shade plugin → fat JAR, mainClass: kfs.sc.sms.SmsApp
```

## Konvence

- Java 21, no Spring/no frameworks (jen jSerialComm, SnakeYAML, SLF4J/Logback)
- Records pro config i data (SmsMessage, AppConfig.*)
- ScheduledExecutorService pro polling loopy
- REST: `POST application/x-www-form-urlencoded`, payload `data=URL_ENCODED_JSON`
- Auth: HTTP Basic Auth
- Update mechanismus: JAR stahne SmsApp-new.jar → exit(42) → .bat swap → restart
- Testy: JUnit 5 + Mockito, bez Spring test context

## Dulezite

- `openModem: false` v application.yml pro lokalni vyvoj (neni serial port)
- AtModemSmsGateway(portName, baudRate) — baudRate z configu
- SmsConfig record ma `outgoingPollIntervalMs` (default 5000)
- shade plugin mainClass = `kfs.sc.sms.SmsApp` (ne Main)
