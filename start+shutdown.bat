```bat
@echo off
setlocal
cd /d "D:\FILES\project\Unofi"

echo ========================================
echo              UNOFI
echo ========================================
echo.

echo [1/3] Commit and push...
git add .
git diff --cached --quiet
if errorlevel 1 (
    git commit -m "chore: update Unofi"
    git push
) else (
    echo No changes to commit.
)

echo.
echo [2/3] Building Unofi...
go build -o unofi.exe .
if errorlevel 1 (
    echo Build failed.
    pause
    exit /b 1
)

echo.
echo [3/3] Starting Unofi...
start "UNOFI SERVER" /b unofi.exe

echo.
echo ========================================
echo UNOFI SERVER RUNNING
echo Press L to shutdown and close.
echo ========================================
echo.

:WAIT
choice /c L /n /t 1 /d L >nul
if errorlevel 1 goto SHUTDOWN
goto WAIT

:SHUTDOWN
echo.
echo Shutting down Unofi...

taskkill /f /im unofi.exe >nul 2>&1

echo Server stopped.
echo Closing...
timeout /t 1 /nobreak >nul
exit
```
