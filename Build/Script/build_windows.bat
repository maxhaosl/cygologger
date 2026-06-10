@echo off
REM Build script for Windows

echo Building CYGoLogger for Windows...

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..\..
set BUILD_DIR=%PROJECT_ROOT%\Build

REM Create build directory
if not exist "%BUILD_DIR%\Bin" mkdir "%BUILD_DIR%\Bin"
if not exist "%BUILD_DIR%\Lib" mkdir "%BUILD_DIR%\Lib"

REM Build library (c-archive)
echo Building Go library...
cd /d "%PROJECT_ROOT%\gologger"
go build -buildmode=c-archive -o "%BUILD_DIR%\Lib\libgologger.a" .

REM Build examples
echo Building simple_example...
go build -o "%BUILD_DIR%\Bin\simple_example.exe" Example/simple_example.go

echo Building simple_logger_test...
go build -o "%BUILD_DIR%\Bin\simple_logger_test.exe" Example/simple_logger_test.go

echo Building console_test...
go build -o "%BUILD_DIR%\Bin\console_test.exe" Example/console_test/main.go

echo Building simple_test...
go build -o "%BUILD_DIR%\Bin\simple_test.exe" Example/simple_test/main.go

echo.
echo Build artifacts:
dir "%BUILD_DIR%\Bin\"
dir "%BUILD_DIR%\Lib\"

echo.
echo Build complete!
