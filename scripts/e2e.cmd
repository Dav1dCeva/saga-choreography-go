@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0e2e.ps1" %*
exit /b %ERRORLEVEL%
