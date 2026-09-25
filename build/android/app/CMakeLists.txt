// 至道选课 Android 壳 CMake：编译 JNI_OnLoad 桥（jni_android.c）成
// libxuanke_jni.so。注意：Go 引擎 libxuanke.so（含导出符号 + JNI_OnLoad）
// 由 CI 用 NDK 交叉编译直接进 jniLibs；本 CMake 目标仅用于 Gradle
// externalNativeBuild 装配校验（壳本身无 C 源码需要链接）。
cmake_minimum_required(VERSION 3.18)
project(xuanke_android_shell)

# Go 编译的 libxuanke.so 经 jniLibs 直接打入 APK（见 build.gradle sourceSets），
# 无需链接本地 C 库；空目标让 externalNativeBuild 校验通过。
add_library(xuanke_shell SHARED stub.c)
