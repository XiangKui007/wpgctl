@echo off
REM 给 Windows 现场/开发机用：绕过 ExecutionPolicy，转调 build.ps1。
REM 仓库根目录执行：  scripts\build.cmd
REM 或：               scripts\build.cmd -Target release
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %*
