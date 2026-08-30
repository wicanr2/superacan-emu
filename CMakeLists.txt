cmake_minimum_required(VERSION 3.20)
project(superacan-emu CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

include(FetchContent)

# Moira（Motorola 68k 模擬核心，MIT，Dirk W. Hoffmann）
# 版本固定紀錄於 README.md
FetchContent_Declare(
    moira
    GIT_REPOSITORY https://github.com/dirkwhoffmann/Moira
    GIT_TAG        a4c273b08e07d82c73289ac032867c845969a0f2
    GIT_SHALLOW    FALSE
)
FetchContent_GetProperties(moira)
if(NOT moira_POPULATED)
    FetchContent_Populate(moira)
    # 只建置 Moira/ 子目錄（CPU 核心程式庫）；repo 根的 Runner/Cputester
    # 依賴 binutils config.h，在本環境無法建置，也不需要。
    add_subdirectory(${moira_SOURCE_DIR}/Moira ${moira_BINARY_DIR})
endif()

# CLK（Thomas Harte，MIT）：取其 WDC 65C02 核心（Processors/6502Mk2，header-only）
# 本里程碑只整合進 build 與建立 Bus 介面 stub，不實際執行副 CPU。
FetchContent_Declare(
    clk
    GIT_REPOSITORY https://github.com/TomHarte/CLK
    GIT_TAG        096de57445920ecf16cf066979422e34aceb843a
    GIT_SHALLOW    FALSE
)
FetchContent_GetProperties(clk)
if(NOT clk_POPULATED)
    FetchContent_Populate(clk)
endif()

add_executable(superacan-emu
    src/main.cpp
    src/bus.cpp
    src/umc6650.cpp
    src/cpu68k.cpp
    src/cpu65c02.cpp
)

target_include_directories(superacan-emu PRIVATE
    src
    ${moira_SOURCE_DIR}
    ${clk_SOURCE_DIR}        # CLK 6502Mk2 以 "Processors/..." 路徑 include
)

target_link_libraries(superacan-emu PRIVATE moira)

# 預留：未來視窗/影像里程碑加 SDL2
# find_package(SDL2)
# target_link_libraries(superacan-emu PRIVATE SDL2::SDL2)
