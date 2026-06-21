# kfsSms — Instalační příručka (Windows 10/11)

SMS gateway pro odesílání a příjem SMS přes USB modem Teltonika TRM250.

---

## Co potřebuješ

- **Windows 10 nebo 11** (notebook nebo PC)
- **Teltonika TRM250** USB modem
- **SIM karta** (aktivní, se zapnutou SMS službou)
- **USB kabel** (datový, ne jen nabíjecí)
- Přístup k internetu (pro stažení programu)

---

## 1. Připojení modemu

### Vložení SIM karty
1. Odpoj modem od USB (pokud je zapojený)
2. Vlož SIM kartu do slotu na modemu (kontakty dolů, zářez podle nákresu)
3. Připoj modem USB kabelem do počítače

### Instalace driverů
- **Windows 10/11** by měl nainstalovat drivery automaticky
- Pokud ne: stáhni z https://wiki.teltonika-networks.com (sekce TRM250 → Downloads → Drivers)
- Po instalaci restartuj počítač

### Zjištění COM portu
1. Klikni pravým na **Start** → **Správce zařízení** (Device Manager)
2. Rozbal **Porty (COM & LPT)**
3. Najdi port s názvem obsahujícím "Quectel" nebo "USB Serial" — např. **COM9**
4. **Zapamatuj si číslo** (COM3, COM9 apod.) — budeš ho potřebovat v konfiguraci

> **Tip:** Pokud modem přepojíš do jiného USB portu, číslo COM se může změnit. Vždy zkontroluj ve Správci zařízení.

---

## 2. Instalace kfsSms

### Ruční instalace (doporučeno)

1. Vytvoř složku `C:\kfsSms`
2. Stáhni nejnovější `kfsSms.exe` z https://github.com/k0fis/kfsSms/releases/latest
3. Ulož do `C:\kfsSms\kfsSms.exe`
4. Vytvoř soubor `C:\kfsSms\config.yml` (viz další krok)
5. Vytvoř soubor `C:\kfsSms\run.bat` (viz krok 4)

### Automatická instalace (PowerShell)

Spusť PowerShell **jako správce** (pravý klik → "Spustit jako správce"):

```powershell
# Vytvoř složku
New-Item -ItemType Directory -Path "C:\kfsSms" -Force

# Stáhni nejnovější .exe z GitHubu
$release = Invoke-RestMethod "https://api.github.com/repos/k0fis/kfsSms/releases/latest"
$asset = $release.assets | Where-Object { $_.name -eq "kfsSms.exe" }
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile "C:\kfsSms\kfsSms.exe"

Write-Host "Staženo: kfsSms.exe" -ForegroundColor Green
```

---

## 3. Konfigurace

Vytvoř soubor `C:\kfsSms\config.yml` s tímto obsahem:

```yaml
sms:
  portName: "COM9"              # ← ZMĚŇ na svůj COM port z kroku 1
  baudRate: 115200
  pollIntervalMs: 5000          # jak často číst SMS z modemu (5 sekund)
  outgoingPollIntervalMs: 5000  # jak často se ptát serveru na SMS k odeslání
  openModem: true
  sendMaxRetries: 3
  sendRetryDelayMs: 5000

api:
  baseUrl: "https://sc.example.com/kfsRealBotSmss"   # ← URL SMS serveru
  user: "sms-user"                                    # ← přihlašovací jméno
  password: "change-me"                               # ← heslo

msisdn:
  pin: ""                       # SIM PIN — prázdné pokud SIM nemá PIN

update:
  owner: "k0fis"
  repo: "kfsSms"
```

### Co upravit:
- **portName** — číslo COM portu z kroku 1 (např. `"COM3"`, `"COM9"`)
- **api.baseUrl** — URL serveru (dostaneš od správce systému)
- **api.user** / **api.password** — přihlašovací údaje (dostaneš od správce)
- **pin** — PIN k SIM kartě, pokud je nastavený (jinak nechej prázdné `""`)

---

## 4. Spuštění

### Ruční test

Otevři příkazový řádek (cmd) nebo PowerShell:

```
cd C:\kfsSms
kfsSms.exe config.yml
```

Mělo by se objevit:
```
kfsSms starting version=1.0.0 port=COM9
checking SIM PIN
SIM already ready
waiting for network registration
network registered                    ← toto může trvat až 2 minuty
modem ready port=COM9
```

Ukonči: **Ctrl+C**

### Automatický start (wrapper skript)

Vytvoř `C:\kfsSms\run.bat`:

```batch
@echo off
:loop
echo [%date% %time%] Starting kfsSms...
C:\kfsSms\kfsSms.exe C:\kfsSms\config.yml
set EC=%ERRORLEVEL%
if %EC%==42 (
    echo Update detected, swapping...
    if exist C:\kfsSms\kfsSms-new.exe (
        del C:\kfsSms\kfsSms.exe
        ren C:\kfsSms\kfsSms-new.exe kfsSms.exe
    )
    goto loop
)
echo Exited with code %EC%, restarting in 10s...
timeout /t 10 /nobreak
goto loop
```

---

## 5. Automatický start při zapnutí PC

### Varianta A: Plánovač úloh (doporučeno)

1. Otevři **Plánovač úloh** (Task Scheduler) — Start → hledej "Plánovač"
2. Klikni **Vytvořit úlohu** (Create Task)
3. Záložka **Obecné**:
   - Název: `kfsSms`
   - Zaškrtni: "Spustit s nejvyššími oprávněními"
   - "Spustit bez ohledu na přihlášení uživatele"
4. Záložka **Aktivační události** (Triggers):
   - Nová → "Při spuštění systému"
5. Záložka **Akce** (Actions):
   - Nová → Program: `C:\kfsSms\run.bat`
   - Spustit v: `C:\kfsSms`
6. Záložka **Podmínky**:
   - Odškrtni "Spouštět jen při napájení ze sítě"
7. Záložka **Nastavení**:
   - Zaškrtni "Při selhání restartovat každé 1 minuty" (max 3×)
   - Odškrtni "Zastavit pokud běží déle než..."
8. OK → zadej heslo uživatele

### Varianta B: Složka Po spuštění (jednodušší, ale méně spolehlivé)

1. Stiskni **Win+R**, napiš `shell:startup`, Enter
2. Do otevřené složky vytvoř zástupce na `C:\kfsSms\run.bat`

---

## 6. Ověření funkce

### Kontrola že běží
- V Plánovači úloh: stav "Běží" u úlohy `kfsSms`
- Nebo ve Správci úloh (Ctrl+Shift+Esc): proces `kfsSms.exe`

### Test odeslání SMS
1. V CRM systému (scReal) vytvoř novou odchozí SMS
2. Do 5 sekund by se měla odeslat (zkontroluj stav v systému: `commit`)

### Test příjmu SMS
1. Pošli SMS na číslo SIM karty v modemu
2. Do 5 sekund by se měla objevit v systému

### Řešení problémů

| Příznak | Příčina | Řešení |
|---------|---------|--------|
| "cannot open port COM9" | Špatný COM port nebo modem nepřipojený | Zkontroluj Správce zařízení |
| "SIM requires PIN" | SIM má PIN a není v configu | Doplň `pin` do config.yml |
| "network registration timeout" | Slabý signál nebo špatná SIM | Zkus jinou polohu modemu, ověř SIM v telefonu |
| "poll: HTTP 401" | Špatné přihlašovací údaje | Zkontroluj user/password v config.yml |
| "poll: HTTP 404" | Špatná URL serveru | Zkontroluj baseUrl |
| Program se restartuje ve smyčce | Modem odpojený nebo chyba config | Podívej se na výpis v cmd okně |

---

## 7. Aktualizace

Program se aktualizuje **automaticky**:
1. Při startu zkontroluje GitHub Releases
2. Pokud je nová verze, stáhne `kfsSms-new.exe`
3. Ukončí se s kódem 42
4. `run.bat` swapne soubory a restartuje

Pro ruční aktualizaci:
1. Stáhni nový `kfsSms.exe` z https://github.com/k0fis/kfsSms/releases/latest
2. Zastav program (Ctrl+C nebo zastav úlohu v Plánovači)
3. Přepiš `C:\kfsSms\kfsSms.exe`
4. Spusť znovu

---

## Technické údaje

| Parametr | Hodnota |
|----------|---------|
| Modem | Teltonika TRM250 (Quectel BG96, LTE Cat-M1) |
| USB Vendor ID | 0x2C7C |
| USB Product ID | 0x0296 |
| Baud rate | 115200 |
| Komunikace | AT příkazy přes sériový port |
| Program | Go, jeden .exe soubor (~10 MB) |
| Vyžaduje | Windows 10/11, USB port |
| Nevyžaduje | Java, .NET, ani žádný runtime |
