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
