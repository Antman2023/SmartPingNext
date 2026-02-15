@echo off
setlocal

echo Building frontend...
cd web
if not exist node_modules npm install
call npm run build
cd ..

echo Copying frontend files...
if exist src\static\html rmdir /s /q src\static\html
mkdir src\static\html
xcopy /e /y web\dist\* src\static\html\

echo Building backend for linux/arm64...
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=1
set CC=zigcc -target aarch64-linux-musl
set CXX=zigcpp -target aarch64-linux-musl

go build -v -o bin/smartping src/smartping.go
echo Build complete!
endlocal
