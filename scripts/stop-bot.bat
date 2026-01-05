@echo off
REM ========================================
REM 飞书机器人停止脚本 (Windows)
REM ========================================

cd /d %~dp0\..

echo ========================================
echo 飞书机器人停止脚本
echo ========================================

REM 检查是否有 bot.exe 进程运行
tasklist /FI "IMAGENAME eq bot.exe" 2>nul | find /I "bot.exe" >nul
if errorlevel 1 (
    echo [!] 没有发现运行中的 bot.exe 进程
    echo.
    pause
    exit /b 0
)

REM 停止所有 bot.exe 进程
echo.
echo [*] 正在停止 bot.exe 进程...
taskkill /F /IM bot.exe >nul 2>&1

REM 等待进程完全退出
echo [*] 等待进程退出...
timeout /t 3 /nobreak >nul

REM 再次检查
tasklist /FI "IMAGENAME eq bot.exe" 2>nul | find /I "bot.exe" >nul
if not errorlevel 1 (
    echo [错误] 进程停止失败！
    echo 请手动检查并结束进程
    pause
    exit /b 1
)

echo [√] 机器人已停止

REM 清理 PID 文件
if exist "bot.pid" (
    del /F /Q bot.pid
)

echo.
echo ========================================
echo 停止完成！
echo ========================================
echo.

pause
