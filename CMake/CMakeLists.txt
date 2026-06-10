cmake_minimum_required(VERSION 3.16)
project(CYGoLogger C)

set(CMAKE_C_STANDARD 11)
set(CMAKE_C_STANDARD_REQUIRED ON)

# Options
option(BUILD_EXAMPLES "Build example programs" ON)

# Detect OS
if(CMAKE_SYSTEM_NAME STREQUAL "Windows")
    set(PLATFORM_WINDOWS 1)
elseif(CMAKE_SYSTEM_NAME STREQUAL "Darwin")
    set(PLATFORM_MACOS 1)
elseif(CMAKE_SYSTEM_NAME STREQUAL "Linux")
    set(PLATFORM_LINUX 1)
endif()

# Output directories
set(CMAKE_RUNTIME_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/Bin)
set(CMAKE_LIBRARY_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/Lib)
set(CMAKE_ARCHIVE_OUTPUT_DIRECTORY ${CMAKE_BINARY_DIR}/Lib)

# Go configuration
find_program(GO_EXECUTABLE go REQUIRED)
message(STATUS "Found Go: ${GO_EXECUTABLE}")
execute_process(
    COMMAND ${GO_EXECUTABLE} version
    OUTPUT_VARIABLE GO_VERSION
    OUTPUT_STRIP_TRAILING_WHITESPACE
)
message(STATUS "Go version: ${GO_VERSION}")

# Get paths relative to CMakeLists.txt location
get_filename_component(GOLOGGER_MODULE_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/../gologger" ABSOLUTE)
get_filename_component(PROJECT_ROOT "${CMAKE_CURRENT_SOURCE_DIR}/.." ABSOLUTE)

message(STATUS "Module root: ${GOLOGGER_MODULE_ROOT}")
message(STATUS "Project root: ${PROJECT_ROOT}")

# For C/C++ interop with -buildmode=c-archive, add a cmd/main.go entry point.
# For Go usage, examples import the module directly via go.mod replace.

# ---- Build examples ----
if(BUILD_EXAMPLES)
    # simple_example
    add_custom_target(simple_example
        COMMAND ${GO_EXECUTABLE} build -o ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/simple_example .
        WORKING_DIRECTORY ${PROJECT_ROOT}/examples/simple_example
        COMMENT "Building simple_example"
        VERBATIM
    )

    # simple_logger_test
    add_custom_target(simple_logger_test
        COMMAND ${GO_EXECUTABLE} build -o ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/simple_logger_test .
        WORKING_DIRECTORY ${PROJECT_ROOT}/examples/simple_logger_test
        COMMENT "Building simple_logger_test"
        VERBATIM
    )

    # console_test
    add_custom_target(console_test
        COMMAND ${GO_EXECUTABLE} build -o ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/console_test .
        WORKING_DIRECTORY ${PROJECT_ROOT}/examples/console_test
        COMMENT "Building console_test"
        VERBATIM
    )

    # simple_test
    add_custom_target(simple_test
        COMMAND ${GO_EXECUTABLE} build -o ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}/simple_test .
        WORKING_DIRECTORY ${PROJECT_ROOT}/examples/simple_test
        COMMENT "Building simple_test"
        VERBATIM
    )

    add_custom_target(examples
        DEPENDS simple_example simple_logger_test console_test simple_test
        COMMENT "All examples built"
    )
endif()

# ---- Print summary ----
message(STATUS "")
message(STATUS "=== CYGoLogger Build Configuration ===")
message(STATUS "  Platform: ${CMAKE_SYSTEM_NAME}")
message(STATUS "  Go executable: ${GO_EXECUTABLE}")
message(STATUS "  Module root: ${GOLOGGER_MODULE_ROOT}")
message(STATUS "  Binaries output: ${CMAKE_RUNTIME_OUTPUT_DIRECTORY}")
message(STATUS "===================================")
