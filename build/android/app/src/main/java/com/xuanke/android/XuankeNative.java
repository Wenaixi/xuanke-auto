package com.xuanke.android;

/**
 * Go 引擎 c-shared 库的 JNI 桥（libxuanke.so 导出函数）。
 * 与 backend/platform_android.go 的 //export 函数一一对应。
 */
public final class XuankeNative {
    static {
        System.loadLibrary("xuanke");
    }

    private XuankeNative() {
    }

    /** 注入沙箱数据目录（filesDir）；必须在 start() 之前调用。 */
    public static native void setDataDir(String filesDir);

    /** 启动选课引擎（API + 调度器 + DB，监听 127.0.0.1:3091）。 */
    public static native void start();

    /** 优雅停止引擎（DB/会话关闭）。 */
    public static native void stop();
}
