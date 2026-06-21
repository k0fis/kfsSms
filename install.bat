@echo off
echo === kfsSms Installer ===
echo.

:: 1. Vytvor slozku
if not exist "C:\kfsSms" (
    mkdir "C:\kfsSms"
    echo Vytvorena slozka C:\kfsSms
) else (
    echo Slozka C:\kfsSms jiz existuje
)

:: 2. Stahni kfsSms.exe z GitHub Releases
echo.
echo Stahuji kfsSms.exe z GitHubu...
powershell -Command "$r = Invoke-RestMethod 'https://api.github.com/repos/k0fis/kfsSms/releases/latest'; $a = $r.assets | Where-Object { $_.name -eq 'kfsSms.exe' }; Invoke-WebRequest -Uri $a.browser_download_url -OutFile 'C:\kfsSms\kfsSms.exe'"
if %ERRORLEVEL% neq 0 (
    echo CHYBA: Nepodarilo se stahnout kfsSms.exe
    echo Stahni rucne z: https://github.com/k0fis/kfsSms/releases/latest
    pause
    exit /b 1
)
echo kfsSms.exe stazen

:: 3. Stahni run.bat
echo.
echo Stahuji run.bat...
powershell -Command "Invoke-WebRequest -Uri 'https://raw.githubusercontent.com/k0fis/kfsSms/main/run.bat' -OutFile 'C:\kfsSms\run.bat'"
if %ERRORLEVEL% neq 0 (
    echo POZOR: run.bat se nepodarilo stahnout
) else (
    echo run.bat stazen
)

:: 4. Vytvor config.yml (pokud neexistuje)
if not exist "C:\kfsSms\config.yml" (
    echo.
    echo Vytvarim config.yml...
    (
        echo # kfsSms konfigurace
        echo # UPRAV PRED SPUSTENIM!
        echo.
        echo sms:
        echo   portName: "COM9"              # ZMEN na svuj COM port ^(viz Spravce zarizeni^)
        echo   baudRate: 115200
        echo   pollIntervalMs: 5000
        echo   outgoingPollIntervalMs: 5000
        echo   openModem: true
        echo   sendMaxRetries: 3
        echo   sendRetryDelayMs: 5000
        echo.
        echo api:
        echo   baseUrl: ""                   # URL serveru ^(zeptej se spravce^)
        echo   user: ""                      # prihlasovaci jmeno
        echo   password: ""                  # heslo
        echo.
        echo msisdn:
        echo   pin: ""                       # SIM PIN ^(prazdne pokud neni^)
        echo.
        echo update:
        echo   owner: "k0fis"
        echo   repo: "kfsSms"
    ) > "C:\kfsSms\config.yml"
    echo config.yml vytvoren
) else (
    echo config.yml jiz existuje, preskakuji
)

:: 5. Otevri config v Notepadu
echo.
echo ============================================
echo   HOTOVO!
echo.
echo   Nyni se otevre config.yml v Notepadu.
echo   UPRAV:
echo     - portName  (COM port modemu)
echo     - baseUrl   (adresa serveru)
echo     - user      (prihlasovaci jmeno)
echo     - password  (heslo)
echo     - pin       (SIM PIN, pokud je)
echo.
echo   Po ulozeni spust: C:\kfsSms\run.bat
echo ============================================
echo.
notepad "C:\kfsSms\config.yml"

pause
