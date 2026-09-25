package com.xuanke.android;

import android.Manifest;
import android.app.Activity;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.os.Build;
import android.os.Bundle;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;

import java.io.File;

/**
 * 主活动：加载 Go 引擎（c-shared libxuanke.so），WebView 指向
 * http://127.0.0.1:PORT/（引擎内嵌完整前端）。与桌面双击 exe 语义一致：
 * 本机 127.0.0.1 服务 + WebView 浏览器，无公网暴露。
 *
 * 启动序列（与 desktop main 对齐）：
 *   1. onCreate → System.loadLibrary("xuanke") 加载 Go 引擎
 *   2. XuankeSetDataDir(filesDir) 注入沙箱数据目录（config.dataDir=filesDir/data）
 *   3. XuankeStart() 启动引擎（API + 调度器 + DB，监听 127.0.0.1:3091）
 *   4. 拉起 EngineService 前台服务保活
 *   5. WebView 加载 http://127.0.0.1:3091/
 */
public class MainActivity extends Activity {

    // 引擎端口（与 backend 默认一致；如需改经 build.gradle 注入）
    static final String ENGINE_URL = "http://127.0.0.1:3091/";

    static {
        System.loadLibrary("xuanke");
    }

    private WebView web;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Android 13+ 前台服务通知需运行时授权（引擎保活通知）
        if (Build.VERSION.SDK_INT >= 33
                && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1);
        }

        // 数据目录注入必须在 XuankeStart（config.Load）之前
        File filesDir = getFilesDir();
        XuankeNative.setDataDir(filesDir.getAbsolutePath());
        XuankeNative.start();

        // 前台服务保活（引擎常驻后台抢课）
        startForegroundService(new Intent(this, EngineService.class));

        web = new WebView(this);
        web.setBackgroundColor(0xFF000000); // 纯黑极简（与前端设计系统一致）
        WebSettings s = web.getSettings();
        s.setJavaScriptEnabled(true);
        s.setDomStorageEnabled(true);
        // 仅加载本机引擎，禁止 WebView 跳转外部（避免被劫持到任意站点）
        web.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String url = request.getUrl().toString();
                if (url.startsWith(ENGINE_URL) || url.equals("about:blank")) {
                    return false; // 放行引擎自身导航
                }
                return true; // 其它一律拦截
            }
        });
        setContentView(web);
        web.loadUrl(ENGINE_URL);
    }

    @Override
    public void onBackPressed() {
        // 返回键=收进后台（不销毁服务，引擎继续常驻）；再次打开回到引擎
        moveTaskToBack(true);
    }

    @Override
    protected void onDestroy() {
        // 活动销毁时优雅停引擎（服务仍在则引擎已随进程结束）
        XuankeNative.stop();
        if (web != null) {
            web.destroy();
        }
        super.onDestroy();
    }
}
