@echo off
setlocal EnableExtensions
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
    if errorlevel 1 (
        echo Commit failed.
        pause
        exit /b 1
    )

    git push
    if errorlevel 1 (
        echo Push failed.
        pause
        exit /b 1
    )
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

for /f %%P in ('powershell -NoProfile -Command "$p=Start-Process -FilePath '.\unofi.exe' -PassThru; $p.Id"') do set "UNOFI_PID=%%P"

echo.
echo ========================================
echo        UNOFI SERVER RUNNING
echo PID: %UNOFI_PID%
echo.
echo Press L to shutdown and close.
echo ========================================
echo.

:WAIT
choice /c LX /n /t 1 /d X >nul

if errorlevel 2 goto WAIT
if errorlevel 1 goto SHUTDOWN

goto WAIT

:SHUTDOWN
echo.
echo Shutting down Unofi...

if defined UNOFI_PID (
    taskkill /f /pid %UNOFI_PID% >nul 2>&1
)

echo Server stopped.
echo Closing...

timeout /t 1 /nobreak >nul
exit /b 0