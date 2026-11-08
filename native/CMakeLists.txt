cmake_minimum_required(VERSION 3.19)
project(terminal_libghostty_native LANGUAGES C)

include(CTest)
include(FetchContent)

# Intended upstream revision for reproducible builds. The available local
# Ghostty source snapshot has no .git metadata, so it cannot be verified against
# this pin; local builds use FETCHCONTENT_SOURCE_DIR_GHOSTTY explicitly.
# libghostty-vt is the supported public C API; ConPTY and rendering are owned here.
set(GHOSTTY_UPSTREAM_REVISION "5813c9461d5af78b6307a1254d00b1bc190eaccd"
    CACHE STRING "Pinned ghostty-org/ghostty revision")

FetchContent_Declare(ghostty
    GIT_REPOSITORY https://github.com/ghostty-org/ghostty.git
    GIT_TAG ${GHOSTTY_UPSTREAM_REVISION}
    GIT_SHALLOW FALSE
)
FetchContent_MakeAvailable(ghostty)

add_executable(ghostty_vt_smoke libghostty_vt_smoke.c)
target_link_libraries(ghostty_vt_smoke PRIVATE ghostty-vt)
target_compile_features(ghostty_vt_smoke PRIVATE c_std_11)

add_library(terminal_libghosty_host STATIC ghostty_host.c)
target_link_libraries(terminal_libghosty_host PUBLIC ghostty-vt)
target_include_directories(terminal_libghosty_host PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(terminal_libghosty_host PUBLIC c_std_11)

if(BUILD_TESTING)
    add_executable(ghostty_host_test ghostty_host_test.c)
    target_link_libraries(ghostty_host_test PRIVATE terminal_libghosty_host)
    target_compile_features(ghostty_host_test PRIVATE c_std_11)
endif()

if(WIN32)
    add_library(terminal_libghosty_conpty STATIC windows_conpty.c)
    target_include_directories(terminal_libghosty_conpty PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
    target_compile_features(terminal_libghosty_conpty PUBLIC c_std_11)
    target_compile_definitions(terminal_libghosty_conpty PUBLIC _WIN32_WINNT=0x0A00)
    target_link_libraries(terminal_libghosty_conpty PUBLIC kernel32)

    if(BUILD_TESTING)
        add_executable(windows_conpty_test windows_conpty_test.c)
        target_link_libraries(windows_conpty_test PRIVATE terminal_libghosty_conpty)
        target_compile_features(windows_conpty_test PRIVATE c_std_11)
    endif()

    foreach(target ghostty_vt_smoke ghostty_host_test)
        if(TARGET ${target})
            add_custom_command(TARGET ${target} POST_BUILD
                COMMAND ${CMAKE_COMMAND} -E copy_if_different
                    "${ghostty_SOURCE_DIR}/zig-out/bin/ghostty-vt.dll"
                    "$<TARGET_FILE_DIR:${target}>/ghostty-vt.dll")
        endif()
    endforeach()
endif()

if(WIN32)
    add_library(terminal_libghosty_session SHARED ssh_terminal.c)
    set_target_properties(terminal_libghosty_session PROPERTIES PREFIX "" OUTPUT_NAME "terminal_libghosty_session")
    target_link_libraries(terminal_libghosty_session PRIVATE
        terminal_libghosty_host terminal_libghosty_conpty)
    target_include_directories(terminal_libghosty_session PUBLIC ${CMAKE_CURRENT_SOURCE_DIR})
    target_compile_features(terminal_libghosty_session PUBLIC c_std_11)
    target_compile_definitions(terminal_libghosty_session PRIVATE
        TERMINAL_LIBGHOSTY_SESSION_EXPORTS _WIN32_WINNT=0x0A00)
    add_custom_command(TARGET terminal_libghosty_session POST_BUILD
        COMMAND ${CMAKE_COMMAND} -E copy_if_different
            "${ghostty_SOURCE_DIR}/zig-out/bin/ghostty-vt.dll"
            "$<TARGET_FILE_DIR:terminal_libghosty_session>/ghostty-vt.dll")

    if(BUILD_TESTING)
        add_executable(ssh_terminal_test ssh_terminal_test.c)
        target_link_libraries(ssh_terminal_test PRIVATE terminal_libghosty_session)
        target_compile_features(ssh_terminal_test PRIVATE c_std_11)
        add_custom_command(TARGET ssh_terminal_test POST_BUILD
            COMMAND ${CMAKE_COMMAND} -E copy_if_different
                "$<TARGET_FILE:terminal_libghosty_session>"
                "$<TARGET_FILE_DIR:ssh_terminal_test>/")
    endif()
endif()

if(BUILD_TESTING)
    add_test(NAME libghostty_vt_smoke COMMAND ghostty_vt_smoke)
    add_test(NAME ghostty_host_bridge COMMAND ghostty_host_test)
    if(WIN32)
        add_test(NAME windows_conpty_smoke COMMAND windows_conpty_test)
        add_test(NAME ssh_conpty_ghostty_e2e COMMAND ssh_terminal_test)
        set_tests_properties(ssh_conpty_ghostty_e2e PROPERTIES TIMEOUT 15)
    endif()
endif()
