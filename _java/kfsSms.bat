@echo off
setlocal

set JAR=SmsApp.jar
set NEW_JAR=SmsApp-new.jar
set JRE=jre\bin\java.exe

:loop
echo [%date% %time%] Starting %JAR% ...
"%JRE%" -jar "%JAR%" config.yml
set EC=%ERRORLEVEL%

if %EC%==42 (
    echo [%date% %time%] Exit code 42 — update detected.
    if exist "%NEW_JAR%" (
        echo [%date% %time%] Swapping %NEW_JAR% -^> %JAR%
        del /f "%JAR%"
        move "%NEW_JAR%" "%JAR%"
        echo [%date% %time%] Swap complete, restarting...
    ) else (
        echo [%date% %time%] WARNING: %NEW_JAR% not found, restarting current JAR.
    )
    goto loop
)

echo [%date% %time%] Exit code %EC%, restarting in 10 seconds...
timeout /t 10 /nobreak >nul
goto loop
