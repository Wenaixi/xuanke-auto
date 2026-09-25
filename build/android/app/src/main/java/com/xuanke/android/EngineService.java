package com.xuanke.android;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;

/**
 * 前台服务：引擎保活。抢课引擎（Go c-shared）随进程常驻，关闭活动页
 * 仍继续提交；前台服务 + 常驻通知让系统不在后台回收进程（抢课窗口期
 * 不能被打断，这是整机的核心诉求）。
 */
public class EngineService extends Service {

    public static final String CHANNEL_ID = "xuanke_engine";

    @Override
    public void onCreate() {
        super.onCreate();
        startForeground();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // 引擎已在 MainActivity 启动（XuankeStart）；服务只为保活，不重复启动。
        // 返回 START_STICKY：被系统回收后自动重建（重建会再 startForeground）。
        return START_STICKY;
    }

    private void startForeground() {
        NotificationManager nm = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationChannel ch = new NotificationChannel(
                    CHANNEL_ID, "选课引擎", NotificationManager.IMPORTANCE_LOW);
            ch.setDescription("抢课引擎后台运行中");
            nm.createNotificationChannel(ch);
        }
        Intent open = new Intent(this, MainActivity.class);
        PendingIntent pi = PendingIntent.getActivity(
                this, 0, open, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
        Notification n = new Notification.Builder(this, CHANNEL_ID)
                .setContentTitle("至道选课")
                .setContentText("引擎后台运行中，选课窗口开启自动提交")
                .setSmallIcon(android.R.drawable.ic_popup_sync)
                .setContentIntent(pi)
                .setOngoing(true)
                .build();
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(1, n, android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC);
        } else {
            startForeground(1, n);
        }
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null; // 非绑定服务
    }
}
