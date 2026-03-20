# kfsSms — Stav projektu

## Co to je

Java 21 aplikace pro Windows notebook s USB 4G modemem Teltonika TRM250.
Bidirectional SMS gateway — prijima SMS z modemu a posilá na server, a zaroven polluje server pro odchozi SMS a posila pres modem.

## Architektura

```
sms-server (Linux :8081)              kfsSms (Windows + TRM250 modem)
─────────────────────────             ────────────────────────────────
GET  /sms/o  →  {id,numb,text}   ←──  OutgoingSmsService (poll, send via modem, confirm/fail)
POST /sms/c  ←  data={id,time}
POST /sms/f  ←  data={id,time,mesg}
POST /sms/i  ←  data={numb,text,time} ──→  SmsDispatchService (modem incoming → queue → REST)
POST /log    ←  data=plain text
```

API format: `POST application/x-www-form-urlencoded`, payload `data=URL_ENCODED_JSON`.
Auth: HTTP Basic Auth (username:password).

## Data flow

### Prichozi SMS (modem → server)
1. `SmsPollingService` cte SMS z modemu (AT+CMGL) kazdych N ms
2. Vlozi do `BlockingQueue<SmsMessage>`
3. `SmsDispatchService` bere z fronty a vola `SmsRestClient.reportIncoming(numb, text, time)`
4. POST /sms/i na server

### Odchozi SMS (server → modem)
1. `OutgoingSmsService` polluje `SmsRestClient.pollOutgoing()` → GET /sms/o
2. Pokud {id, numb, text} → `gateway.sendSms(numb, text)`
3. OK → `client.confirmSent(id)` (POST /sms/c)
4. Fail → `client.reportFail(id, error)` (POST /sms/f)

## Update mechanismus

1. `GitHubUpdater.updateIfAvailable()` — GitHub API, porovna verze
2. Stahne novy JAR do `SmsApp-new.jar`
3. `SmsApp` zavola `System.exit(42)`
4. `kfsSms.bat` wrapper detekuje exit code 42:
   - Smaze stary `SmsApp.jar`
   - Prejmenuje `SmsApp-new.jar` → `SmsApp.jar`
   - Restartuje
5. Jiny exit code → restart po 10s (auto-recovery)

## Windows distribuce

### install.ps1 (jednorazova instalace)
1. Vytvori `C:\kfsSms`
2. Stahne Temurin JRE 21 ZIP z Adoptium API
3. Stahne SmsApp JAR z GitHub Releases
4. Vytvori sablonu `config.yml`
5. Registruje Task Scheduler: at startup, SYSTEM, highest privileges

### kfsSms.bat (wrapper)
- Nekonecny loop: spusti JAR → zpracuje exit code → restart
- Exit 42 = update swap
- Cokoliv jineho = restart po 10s

## CI/CD

GitHub Actions (`ci.yml`):
- Push na main/develop → build + test
- Tag `v*` → build, upload JAR artifact, create GitHub Release

## Config (application.yml)

```yaml
sms:
  portName: "COM3"          # serial port modemu
  baudRate: 115200           # baud rate
  pollIntervalMs: 5000       # jak casto cist SMS z modemu
  outgoingPollIntervalMs: 5000  # jak casto pollovat server pro odchozi
  openModem: true            # false = neotvira port (dev mode)
  sendMaxRetries: 3
  sendRetryDelayMs: 5000

api:
  baseUrl: "https://server:8081"
  user: "sms-user"
  password: "heslo"

cfg:
  terminate: "quit"          # slovo pro manualni zastaveni z konzole

msisdn:
  pin: "1234"                # SIM PIN

logging:
  level: "INFO"
  packages:
    kfs.sc.sms: DEBUG
```

## Co je hotovo

- [x] AT modem komunikace (jSerialComm) — open, sendSms, readAll, delete, SIM PIN
- [x] SmsPollingService — modem → queue
- [x] SmsDispatchService — queue → REST (reportIncoming)
- [x] OutgoingSmsService — REST → modem (pollOutgoing → sendSms → confirm/fail)
- [x] SmsRestClient — plna implementace (pollOutgoing, reportIncoming, confirmSent, reportFail, sendLog)
- [x] GitHubUpdater — download SmsApp-new.jar, exit(42) pro wrapper
- [x] kfsSms.bat — Windows wrapper loop
- [x] install.ps1 — PowerShell installer
- [x] CI/CD — GitHub Actions build + release (bez jpackage EXE)
- [x] AppConfig — YAML config s nested records
- [x] 36 unit testu (JUnit 5 + Mockito)

## Co chybi / TODO

- [ ] Integracni test s realnym modemem
- [ ] Healthcheck endpoint / monitoring
- [ ] Log rotace na Windows (logback config)
- [ ] Sifrovani credentials v config.yml
- [ ] Retry logika v OutgoingSmsService (momentalne single attempt per poll)
