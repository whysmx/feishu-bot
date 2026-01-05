@echo off
REM ========================================
REM 飞书机器人日志查看脚本 (Windows)
REM ========================================

cd /d %~dp0\..

REM 检查日志文件是否存在
if not exist "logs\bot.log" (
    echo [!] 日志文件不存在: logs\bot.log
    echo.
    echo 请先启动机器人
    pause
    exit /b 0
)

echo ========================================
echo 飞书机器人日志查看
echo ========================================
echo.
echo 显示最后 50 行日志:
echo ----------------------------------------
powershell -Command "Get-Content logs\bot.log -Tail 50"
echo.
echo.
echo ========================================
echo 实时查看日志:
echo   PowerShell: Get-Content logs\bot.log -Wait
echo 或使用文本编辑器打开 logs\bot.log
echo ========================================
echo.

pause
