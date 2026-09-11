@echo off
setlocal
set "SETUP_DIR=%~dp0"
set "SETUP_EXE=%SETUP_DIR%ClientInteractionCRM-Setup-windows-amd64.exe"

if not exist "%SETUP_EXE%" (
  echo Client Interaction CRM Setup was not found:
  echo %SETUP_EXE%
  pause
  exit /b 1
)

"%SETUP_EXE%"
set "SETUP_EXIT=%ERRORLEVEL%"
if not "%SETUP_EXIT%"=="0" pause
endlocal & exit /b %SETUP_EXIT%
