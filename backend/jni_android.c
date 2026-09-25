// jni_android.c —— Go c-shared 库的 JNI 注册桥（Android 平台专用）。
//
// 背景：Go 的 //export 函数符号名是 `XuankeSetDataDir` 等裸名（非 JVM 默认解析的
// `Java_com_xuanke_android_...`），System.loadLibrary 后 JVM 找不到 native 方法，
// 必须由 JNI_OnLoad 里 RegisterNatives 显式把 Go 符号注册到 Java 侧声明的 native 方法。
//
// 符号约定（与 Java 侧 XuankeNative.java 一一对应）：
//   Java:    native void setDataDir(String)   → Go: void XuankeSetDataDir(char*)
//   Java:    native void start()              → Go: void XuankeStart()
//   Java:    native void stop()               → Go: void XuankeStop()
//
// Go c-shared 导出的函数都是 C ABI（extern "C" 等价），本文件用声明 + RegisterNatives
// 建立 JNI 方法表。

//go:build android

#include <jni.h>
#include <stdlib.h>
#include <string.h>

// Go 导出符号声明（与 backend/platform_android.go 的 //export 对应）
extern void XuankeSetDataDir(const char *dir);
extern void XuankeStart(void);
extern void XuankeStop(void);

static void setDataDir(JNIEnv *env, jclass clazz, jstring path) {
    const char *cpath = (*env)->GetStringUTFChars(env, path, NULL);
    XuankeSetDataDir(cpath);
    (*env)->ReleaseStringUTFChars(env, path, cpath);
}

static void start(JNIEnv *env, jclass clazz) {
    XuankeStart();
}

static void stop(JNIEnv *env, jclass clazz) {
    XuankeStop();
}

// 方法表：Java native 方法 → C 桩 → Go 导出
static const JNINativeMethod methods[] = {
    {"setDataDir", "(Ljava/lang/String;)V", (void *)setDataDir},
    {"start",      "()V",                   (void *)start},
    {"stop",       "()V",                   (void *)stop},
};

JNIEXPORT jint JNICALL JNI_OnLoad(JavaVM *vm, void *reserved) {
    JNIEnv *env = NULL;
    if ((*vm)->GetEnv(vm, (void **)&env, JNI_VERSION_1_6) != JNI_OK) {
        return JNI_ERR;
    }
    jclass cls = (*env)->FindClass(env, "com/xuanke/android/XuankeNative");
    if (cls == NULL) {
        return JNI_ERR;
    }
    if ((*env)->RegisterNatives(env, cls, methods,
                                sizeof(methods) / sizeof(methods[0])) < 0) {
        return JNI_ERR;
    }
    return JNI_VERSION_1_6;
}
