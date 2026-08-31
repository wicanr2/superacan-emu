# Deprecated C++ reference implementation; production development is pure Go.
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

# CLK（Thomas Harte，MIT）：取其 WDC 65C02 核心（Processors/6502Mk2，header-only）；
# 副 CPU 已實際執行音效驅動、手把掃描、IRQ/NMI 與 UM6619 通訊。
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
    src/video/um6618.cpp
    src/audio/um6619.cpp
    src/state.cpp
)

target_include_directories(superacan-emu PRIVATE
    src
    ${moira_SOURCE_DIR}
    ${clk_SOURCE_DIR}        # CLK 6502Mk2 以 "Processors/..." 路徑 include
)

target_link_libraries(superacan-emu PRIVATE moira)

# SDL2（視窗輸出）：優先系統套件；否則用 third_party/sdl2-local（由系統
# libsdl2-dev .deb 解出的 headers + 系統 runtime lib，本機無 sudo 時的做法，
# 見 README「建置」）；再不行才 FetchContent 自建（需 X11 dev headers）
find_package(SDL2 QUIET)
if(SDL2_FOUND)
    target_link_libraries(superacan-emu PRIVATE SDL2::SDL2)
elseif(EXISTS "${CMAKE_SOURCE_DIR}/third_party/sdl2-local/include/SDL2/SDL.h")
    message(STATUS "使用 third_party/sdl2-local 的 SDL2 headers + 系統 runtime")
    target_include_directories(superacan-emu PRIVATE
        third_party/sdl2-local/include        # SDL_config.h 以 <SDL2/...> 引用
        third_party/sdl2-local/include/SDL2)  # <SDL.h>
    target_link_directories(superacan-emu PRIVATE third_party/sdl2-local)
    target_link_libraries(superacan-emu PRIVATE SDL2)
else()
    message(STATUS "系統無 SDL2，以 FetchContent 建置 SDL 2.30.0")
    set(SDL_SHARED ON CACHE BOOL "" FORCE)
    set(SDL_STATIC OFF CACHE BOOL "" FORCE)
    set(SDL_TEST OFF CACHE BOOL "" FORCE)
    FetchContent_Declare(
        sdl2
        GIT_REPOSITORY https://github.com/libsdl-org/SDL
        GIT_TAG        release-2.30.0
        GIT_SHALLOW    TRUE
    )
    FetchContent_MakeAvailable(sdl2)
    target_link_libraries(superacan-emu PRIVATE SDL2::SDL2)
endif()
