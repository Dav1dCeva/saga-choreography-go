@echo off
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0saga.ps1" %*
exit /b %ERRORLEVEL%
