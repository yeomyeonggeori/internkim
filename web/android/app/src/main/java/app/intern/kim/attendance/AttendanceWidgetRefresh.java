package app.intern.kim.attendance;

import android.app.AlarmManager;
import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.content.ComponentName;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import android.os.SystemClock;
import android.util.SizeF;
import android.widget.RemoteViews;
import androidx.annotation.Nullable;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

final class AttendanceWidgetRefresh {

    private static final long whileIdle = 30 * 60 * 1000L;
    private static final long minuteMillis = 60 * 1000L;
    private static final long halfMinuteMillis = minuteMillis / 2;
    private static final long staleRedrawnAfter = 20 * 1000L;

    private AttendanceWidgetRefresh() {}

    static void redrawNow(Context context, boolean isTick) {
        AppWidgetManager manager = AppWidgetManager.getInstance(context);
        int[] widgetIDs = manager.getAppWidgetIds(new ComponentName(context, AttendanceWidgetProvider.class));
        long now = System.currentTimeMillis();
        AttendanceWidgetEntry entry = AttendanceWidgetReader.read(context, now, isTick);
        if (!isTick) AttendanceOngoingNotification.update(context, entry);
        if (widgetIDs.length == 0) {
            cancelRedraw(context);
            return;
        }
        String refusal = AttendanceRefusal.recent(context, now);
        Long choiceCloses = AttendanceLocationChoice.closes(context, now);
        boolean isChoosingLocation = choiceCloses != null && !entry.today.isWorking();

        for (int widgetID : widgetIDs) {
            List<AttendanceWidgetSize> sizes = AttendanceWidgetSize.of(manager.getAppWidgetOptions(widgetID));
            manager.updateAppWidget(widgetID, viewsFor(context, entry, widgetID, sizes, refusal, isChoosingLocation));
        }

        long refreshAt = now + whileIdle;
        if (entry.isStale) refreshAt = Math.min(refreshAt, now + staleRedrawnAfter);
        if (refusal != null) refreshAt = Math.min(refreshAt, now + AttendanceRefusal.shownFor);
        if (isChoosingLocation) refreshAt = Math.min(refreshAt, choiceCloses);
        long tickAt = entry.today.isWorking() ? now + untilNextMinute(entry.today.elapsedMillis(now)) : Long.MAX_VALUE;
        cancelRedraw(context);
        if (tickAt < refreshAt) {
            schedule(context, tickAt - now, AttendanceWidgetProvider.actionTick);
        } else {
            schedule(context, refreshAt - now, AttendanceWidgetProvider.actionRedraw);
        }
    }

    private static RemoteViews viewsFor(
        Context context,
        AttendanceWidgetEntry entry,
        int widgetID,
        List<AttendanceWidgetSize> sizes,
        @Nullable String refusal,
        boolean isChoosingLocation
    ) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.S || sizes.size() == 1) {
            return AttendanceWidgetDrawing.views(context, entry, widgetID, sizes.get(0), refusal, isChoosingLocation);
        }
        Map<SizeF, RemoteViews> bySize = new HashMap<>();
        for (AttendanceWidgetSize size : sizes) {
            bySize.put(size.asSize(), AttendanceWidgetDrawing.views(context, entry, widgetID, size, refusal, isChoosingLocation));
        }
        return new RemoteViews(bySize);
    }

    static long untilNextMinute(long elapsedMillis) {
        long untilRoundingTurns = (minuteMillis + halfMinuteMillis - elapsedMillis % minuteMillis) % minuteMillis;
        return untilRoundingTurns == 0 ? minuteMillis : untilRoundingTurns;
    }

    static void cancelRedraw(Context context) {
        alarms(context).cancel(redrawIntent(context, AttendanceWidgetProvider.actionTick));
        alarms(context).cancel(redrawIntent(context, AttendanceWidgetProvider.actionRedraw));
    }

    private static void schedule(Context context, long after, String action) {
        alarms(context).set(AlarmManager.ELAPSED_REALTIME, SystemClock.elapsedRealtime() + after, redrawIntent(context, action));
    }

    private static AlarmManager alarms(Context context) {
        return (AlarmManager) context.getSystemService(Context.ALARM_SERVICE);
    }

    private static PendingIntent redrawIntent(Context context, String action) {
        Intent intent = new Intent(context, AttendanceWidgetProvider.class).setAction(action);
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}
