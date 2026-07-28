@echo off
setlocal EnableExtensions

set "PROJECT_ROOT=%~dp0"
set "FRONTEND_DIR=%PROJECT_ROOT%huawei-ui"
set "BACKEND_DIR=%PROJECT_ROOT%huawei-go"

cd /d "%FRONTEND_DIR%"
if errorlevel 1 exit /b 1
call npm ci
if errorlevel 1 exit /b 1
call npm run build
if errorlevel 1 exit /b 1

if exist "%BACKEND_DIR%\static" rmdir /s /q "%BACKEND_DIR%\static"
mkdir "%BACKEND_DIR%\static"
xcopy "%FRONTEND_DIR%\dist\*" "%BACKEND_DIR%\static\" /E /Y /I
if errorlevel 1 exit /b 1

cd /d "%BACKEND_DIR%"
if errorlevel 1 exit /b 1
go test ./...
if errorlevel 1 exit /b 1
go build -trimpath -o server.exe .
if errorlevel 1 exit /b 1

server.exe
