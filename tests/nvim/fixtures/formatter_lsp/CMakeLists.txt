cmake_minimum_required(VERSION 3.20)
project(FormatterSmoke LANGUAGES C)
add_executable(formatter_smoke main.c)
target_compile_features(formatter_smoke PRIVATE c_std_11)
