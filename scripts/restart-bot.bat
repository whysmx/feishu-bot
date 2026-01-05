@echo off
REM ========================================
REM 飞书机器人重启脚本 (Windows)
REM ========================================

cd /d %~dpq

echo ========================================
echo 飞书机器人重启脚本
echo ========================================

REM 停止机器人
echo.
echo [1/2] 正在停止机器人...
call stop-bot.bat

REM 等待
timeout /t 2 /nobreak >nul

REM 启动机器人
echo.
echo [2/2] 正在启动机器人...
call start-bot.bat
