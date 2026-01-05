@echo off
REM ========================================
REM 飞书机器人启动脚本 (Windows)
REM ========================================

cd /d %~dp0\..

echo ========================================
echo 飞书机器人启动脚本
echo ========================================

REM 检查 .env 文件是否存在
if not exist ".env" (
    echo [错误] .env 文件不存在！
    echo 请先创建 .env 文件并配置必要的环境变量
    echo.
    echo 可以复制示例文件：
    echo   copy .env.example .env
    echo.
    pause
    exit /b 1
)

REM 检查 Go 环境
where go >nul 2>&1
if errorlevel 1 (
    echo [错误] 未找到 Go 环境！
    echo 请先安装 Go: https://golang.org/dl/
    pause
    exit /b 1
)

REM 编译项目
echo.
echo [1/3] 正在编译项目...
go build -o bot.exe cmd/bot/main.go
if errorlevel 1 (
    echo [错误] 编译失败！
    pause
    exit /b 1
)
echo [√] 编译成功

REM 停止旧进程
echo.
echo [2/3] 检查是否有旧进程运行...
tasklist /FI "IMAGENAME eq bot.exe" 2>nul | find /I "bot.exe" >nul
if not errorlevel 1 (
    echo [!] 发现旧进程，正在停止...
    taskkill /F /IM bot.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
    echo [√] 旧进程已停止
) else (
    echo [√] 没有旧进程运行
)

REM 启动新进程
echo.
echo [3/3] 正在启动机器人...
start /B bot.exe > logs\bot.log 2>&1

REM 等待启动
timeout /t 3 /nobreak >nul

REM 检查进程是否启动成功
tasklist /FI "IMAGENAME eq bot.exe" 2>nul | find /I "bot.exe" >nul
if errorlevel 1 (
    echo [错误] 机器人启动失败！
    echo 请检查 logs\bot.log 查看错误信息
    pause
    exit /b 1
)

echo [√] 机器人启动成功！
echo.
echo ========================================
echo 机器人已启动，PID:
tasklist /FI "IMAGENAME eq bot.exe" | find "bot.exe"
echo ========================================
echo.
echo 查看日志:
echo   tail -f logs\bot.log
echo.
echo 停止机器人:
echo   scripts\stop-bot.bat
echo.
echo 重启机器人:
echo   scripts\restart-bot.bat
echo.

REM 保存 PID
for /f "tokens=2" %%a in ('tasklist /FI "IMAGENAME eq bot.exe" ^| find "bot.exe"') do (
    set PID=%%a
)
echo %PID% > bot.pid

echo 启动完成！按任意键关闭此窗口...
pause >nul
