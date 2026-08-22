# kfsSms — Stav projektu

## Aktuální verze
- **v0.1.4** (2026-08-22)
- Go 1.25, single binary, linux/arm64
- GitHub: https://github.com/k0fis/kfsSms

## Produkční prostředí

### kfsSms.local (Raspberry Pi, aarch64)
- Binary: `/opt/kfsSms/kfsSms`
- Config: `/opt/kfsSms/config.yml`
- Systemd: `kfsSms.service` (system-level, root)
- Modem: Teltonika TRM250, `/dev/ttyUSB2`, T-Mobile CZ
- Auto-update: cron `0 4 * * *` → `/opt/kfsSms/update.sh`
- Web log viewer: `http://kfssms.local/` (port 80)
- SSH: `kfssms` alias (user kofis, has sudo)

### SC server (sc.kofis.eu) — kofis-sms-server
- Java standalone JAR, embedded com.sun.net.httpserver
- Port 8082, Apache proxy: `https://robot.soccerreality.cz/kofis-sms`
- Systemd: user-level `kofis-sms-server.service` (user kofis, linger=yes)
- Args: `--port=8082 --send-hour-from=8 --send-hour-to=20`
- DB: `jdbc:postgresql://localhost:5433/kofis`, schema `realbot`
- SSL: Let's Encrypt, certbot.timer (auto-renewal)

## Architektura

```
kofis-web (CRM) → sms_send tabulka
                         ↓
kofis-sms-server (SC:8082) ← kfsSms polls /sms/o
                         ↓
kfsSms (kfssms.local) → TRM250 modem → SMS doručena
                         ↑
příchozí SMS → modem → kfsSms → POST /sms/i → kofis-sms-server → sms_income
```

## Funkce kfsSms (Go binary)
- Bidirectional SMS: poll outgoing + report incoming
- UCS-2 encoding pro non-ASCII (česká diakritika)
- Bare OK acceptance (prevence duplicitních SMS)
- AT response trimming
- Auto-update z GitHub Releases (built-in updater.go + update.sh)
- Web log viewer na port 80 (journalctl + service status)
- SIM PIN unlock, network registration wait (CEREG/CREG, 2min timeout)
- Modem auto-detect (scan serial ports)

## Funkce kofis-sms-server (Java)
- REST API: `/sms/o` (pop outgoing), `/sms/i` (report incoming), `/sms/c` (confirm sent)
- `/health` endpoint (no auth)
- `/log` endpoint (activity log)
- Basic Auth against `kfs_users` table
- sms_sender filtering: contact stays with same phone number
- HTML entity sanitization (&nbsp; etc.)
- Send interval: configurable via CLI args

## CI/CD
- GitHub Actions: on tag push `v*` → build linux/arm64 → release asset `kfsSms`
- Auto-update: kfsSms.local checks daily at 4:00, downloads + stops + swaps + starts

## Config (kfsSms.local)
```yaml
sms:
  portName: /dev/ttyUSB2
  baudRate: 115200
  pollIntervalMs: 5000
  outgoingPollIntervalMs: 5000
  openModem: true
  sendMaxRetries: 3
  sendRetryDelayMs: 5000
api:
  baseUrl: "https://robot.soccerreality.cz/kofis-sms"
  user: "walley"
  password: "sms2026"
msisdn:
  pin: "2562"
update:
  owner: "k0fis"
  repo: "kfsSms"
web:
  port: 80
  enabled: true
```

## Historie

| Datum | Verze | Změna |
|-------|-------|-------|
| 2026-08-22 | v0.1.4 | Web log viewer (port 80) |
| 2026-08-22 | v0.1.3 | Simplified CI (single arm64), fixed update.sh |
| 2026-08-22 | v0.1.2 | UCS-2 support, bare OK fix, AT trim |
| 2026-08-22 | v0.0.2 | Initial Go rewrite deployed to kfssms.local |

## TODO
- [ ] Přepojit Apache `/kfsRealBotSmss` na nový server a zrušit legacy Tomcat WAR
- [ ] HTTPS na kfsSms.local log viewer (nebo VPN-only přístup)
- [ ] Monitoring/alerting při výpadku modem connection
