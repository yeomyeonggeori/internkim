package app.intern.kim.attendance;

import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

public class AttendanceWidgetProvider extends AppWidgetProvider {

    static final String actionClockIn = "app.intern.kim.attendance.CLOCK_IN";
    static final String actionClockOut = "app.intern.kim.attendance.CLOCK_OUT";
    static final String actionShowLocations = "app.intern.kim.attendance.SHOW_LOCATIONS";
    static final String actionHideLocations = "app.intern.kim.attendance.HIDE_LOCATIONS";
    static final String actionRedraw = "app.intern.kim.attendance.REDRAW";
    static final String locationExtra = "location";

    private static final ExecutorService worker = Executors.newSingleThreadExecutor();

    static void redrawSoon(Context context) {
        Context application = context.getApplicationContext();
        worker.execute(() -> AttendanceWidgetRefresh.redrawNow(application));
    }

    @Override
    public void onReceive(Context context, Intent intent) {
        String action = intent.getAction();
        if (actionClockIn.equals(action)) {
            inBackground(() -> AttendanceClockPress.press(context, AttendanceRow.clockIn, intent.getStringExtra(locationExtra)));
        } else if (actionClockOut.equals(action)) {
            inBackground(() -> AttendanceClockPress.press(context, AttendanceRow.clockOut, null));
        } else if (actionShowLocations.equals(action)) {
            inBackground(() -> {
                long now = System.currentTimeMillis();
                AttendanceLocationChoice.open(context, now);
                AttendanceWidgetCacheStore.markTap(context, now);
                AttendanceWidgetRefresh.redrawNow(context);
            });
        } else if (actionHideLocations.equals(action)) {
            inBackground(() -> {
                AttendanceLocationChoice.close(context);
                AttendanceWidgetCacheStore.markTap(context, System.currentTimeMillis());
                AttendanceWidgetRefresh.redrawNow(context);
            });
        } else if (actionRedraw.equals(action)) {
            inBackground(() -> AttendanceWidgetRefresh.redrawNow(context));
        } else {
            super.onReceive(context, intent);
        }
    }

    @Override
    public void onUpdate(Context context, AppWidgetManager manager, int[] widgetIDs) {
        inBackground(() -> AttendanceWidgetRefresh.redrawNow(context));
    }

    @Override
    public void onAppWidgetOptionsChanged(Context context, AppWidgetManager manager, int widgetID, Bundle options) {
        inBackground(() -> AttendanceWidgetRefresh.redrawNow(context));
    }

    @Override
    public void onDeleted(Context context, int[] widgetIDs) {
        for (int widgetID : widgetIDs) AttendanceWidgetStore.forgetWorkplace(context, widgetID);
    }

    @Override
    public void onDisabled(Context context) {
        AttendanceWidgetRefresh.cancelRedraw(context);
    }

    private void inBackground(Runnable work) {
        PendingResult pending = goAsync();
        worker.execute(() -> {
            try {
                work.run();
            } finally {
                pending.finish();
            }
        });
    }
}
