@echo off
setlocal EnableExtensions

cd /d "D:\FILES\project\Unofi"

echo ========================================
echo UNOFI
echo ========================================
echo.

echo [1/3] Updating repository...

git add .
git diff --cached --quiet

if errorlevel 1 (
git commit -m "chore: update Unofi"
if errorlevel 1 (
echo.
echo Commit failed.
pause
exit /b 1
)

git push
if errorlevel 1 (
    echo.
    echo Push failed.
    pause
    exit /b 1
)

) else (
echo No changes to commit.
)

echo.
echo [2/3] Building Unofi...

if exist "cmd\unofi\main.go" (
echo Build target: cmd\unofi
go build -o unofi.exe ./cmd/unofi
) else (
echo Build target: repository root
go build -o unofi.exe .
)

if errorlevel 1 (
echo.
echo Build failed.
pause
exit /b 1
)

if not exist "unofi.exe" (
echo.
echo Build completed but unofi.exe was not created.
pause
exit /b 1
)

echo.
echo [3/3] Starting Unofi...

for /f %%P in ('powershell -NoProfile -Command "$p=Start-Process -FilePath ''.\unofi.exe'' -PassThru; $p.Id"') do set "UNOFI_PID=%%P"

if not defined UNOFI_PID (
echo.
echo Failed to start Unofi.
pause
exit /b 1
)

echo.
echo ========================================
echo UNOFI SERVER RUNNING
echo PID: %UNOFI_PID%
echo.
echo Press L to shutdown and close.
echo ========================================
echo.


choice /c L /n >nul
goto SHUTDOWN


echo.
echo Shutting down Unofi...

if defined UNOFI_PID (
taskkill /f /pid %UNOFI_PID% >nul 2>&1
)

echo Server stopped.
echo Closing...

timeout /t 1 /nobreak >nul
exit /b 0