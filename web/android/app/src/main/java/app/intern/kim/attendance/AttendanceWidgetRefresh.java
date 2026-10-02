package app.intern.kim.attendance;

import android.app.AlarmManager;
import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.os.SystemClock;

final class AttendanceWidgetRefresh {

    private static final long whileIdle = 30 * 60 * 1000L;
    private static final long whileWorking = 15 * 60 * 1000L;
    private static final long staleRedrawnAfter = 20 * 1000L;

    private AttendanceWidgetRefresh() {}

    static void redrawNow(Context context) {
        AppWidgetManager manager = AppWidgetManager.getInstance(context);
        int[] widgetIDs = manager.getAppWidgetIds(new ComponentName(context, AttendanceWidgetProvider.class));
        if (widgetIDs.length == 0) {
            cancelRedraw(context);
            return;
        }

        long now = System.currentTimeMillis();
        AttendanceWidgetEntry entry = AttendanceWidgetReader.read(context, now);
        String refusal = AttendanceRefusal.recent(context, now);
        Long choiceCloses = AttendanceLocationChoice.closes(context, now);
        boolean isChoosingLocation = choiceCloses != null && !entry.today.isWorking();

        for (int widgetID : widgetIDs) {
            manager.updateAppWidget(
                widgetID,
                AttendanceWidgetDrawing.views(context, entry, widgetID, manager.getAppWidgetOptions(widgetID), refusal, isChoosingLocation)
            );
        }

        long redrawAt = now + (entry.today.isWorking() ? whileWorking : whileIdle);
        if (entry.isStale) redrawAt = Math.min(redrawAt, now + staleRedrawnAfter);
        if (refusal != null) redrawAt = Math.min(redrawAt, now + AttendanceRefusal.shownFor);
        if (isChoosingLocation) redrawAt = Math.min(redrawAt, choiceCloses);
        scheduleRedraw(context, redrawAt - now);
    }

    static void cancelRedraw(Context context) {
        alarms(context).cancel(redrawIntent(context));
    }

    private static void scheduleRedraw(Context context, long after) {
        alarms(context).set(AlarmManager.ELAPSED_REALTIME, SystemClock.elapsedRealtime() + after, redrawIntent(context));
    }

    private static AlarmManager alarms(Context context) {
        return (AlarmManager) context.getSystemService(Context.ALARM_SERVICE);
    }

    private static PendingIntent redrawIntent(Context context) {
        Intent intent = new Intent(context, AttendanceWidgetProvider.class).setAction(AttendanceWidgetProvider.actionRedraw);
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}
