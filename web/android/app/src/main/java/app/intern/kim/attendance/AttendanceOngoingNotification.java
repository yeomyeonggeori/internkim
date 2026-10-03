package app.intern.kim.attendance;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import android.os.Build;
import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;
import androidx.core.app.NotificationManagerCompat;
import app.intern.kim.MainActivity;
import app.intern.kim.R;

final class AttendanceOngoingNotification {

    static final String clockInAtExtra = "clockInAt";

    private static final String channelID = "attendance_ongoing";
    private static final int notificationID = 4201;

    private AttendanceOngoingNotification() {}

    static boolean shows(AttendanceToday today, @Nullable Long dismissedClockInAt) {
        AttendanceToday.Segment active = today.activeSegment();
        if (active == null) return false;
        return dismissedClockInAt == null || dismissedClockInAt != active.clockInAt;
    }

    static void update(Context context, AttendanceWidgetEntry entry) {
        if (AttendanceWidgetStore.credential(context) == null) {
            cancel(context);
            return;
        }
        if (entry.failure != null) return;
        if (!shows(entry.today, AttendanceWidgetStore.dismissedOngoing(context))) {
            cancel(context);
            return;
        }
        ensureChannel(context);
        if (NotificationManagerCompat.from(context).areNotificationsEnabled()) {
            NotificationManagerCompat.from(context).notify(notificationID, build(context, entry));
        }
    }

    static void cancel(Context context) {
        NotificationManagerCompat.from(context).cancel(notificationID);
    }

    private static Notification build(Context context, AttendanceWidgetEntry entry) {
        AttendanceToday today = entry.today;
        AttendanceToday.Segment active = today.activeSegment();
        String location = today.location();
        String title = location == null
            ? context.getString(R.string.attendance_widget_working)
            : context.getString(R.string.attendance_ongoing_title, location);
        NotificationCompat.Builder builder = new NotificationCompat.Builder(context, channelID)
            .setSmallIcon(R.drawable.attendance_widget_icon_clock_in)
            .setContentTitle(title)
            .setContentText(context.getString(R.string.attendance_ongoing_clocked_in, active.clockInTime))
            .setWhen(entry.date - today.elapsedMillis(entry.date))
            .setShowWhen(true)
            .setUsesChronometer(true)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_STOPWATCH)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setContentIntent(openApp(context))
            .setDeleteIntent(dismissed(context, active.clockInAt))
            .addAction(
                R.drawable.attendance_widget_icon_mark_out,
                context.getString(R.string.attendance_widget_clock_out),
                AttendanceWidgetProvider.broadcast(context, AttendanceWidgetProvider.actionClockOut)
            );
        return builder.setRequestPromotedOngoing(true).build();
    }

    private static void ensureChannel(Context context) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return;
        NotificationChannel channel = new NotificationChannel(
            channelID,
            context.getString(R.string.attendance_ongoing_channel),
            NotificationManager.IMPORTANCE_DEFAULT
        );
        channel.setSound(null, null);
        channel.enableVibration(false);
        channel.setLockscreenVisibility(Notification.VISIBILITY_PUBLIC);
        context.getSystemService(NotificationManager.class).createNotificationChannel(channel);
    }

    private static PendingIntent openApp(Context context) {
        Intent intent = new Intent(context, MainActivity.class).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
        return PendingIntent.getActivity(context, notificationID, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    private static PendingIntent dismissed(Context context, long clockInAt) {
        Intent intent = new Intent(context, AttendanceWidgetProvider.class)
            .setAction(AttendanceWidgetProvider.actionOngoingDismissed)
            .putExtra(clockInAtExtra, clockInAt);
        return PendingIntent.getBroadcast(context, notificationID, intent, PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }
}
