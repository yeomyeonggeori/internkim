package app.intern.kim.attendance;

import android.content.Context;
import android.content.SharedPreferences;
import androidx.annotation.Nullable;

final class AttendanceRefusal {

    static final long shownFor = 15 * 1000L;

    private static final String messageKey = "widget.refusal.message";
    private static final String momentKey = "widget.refusal.moment";

    private AttendanceRefusal() {}

    static void keep(Context context, String message, long moment) {
        AttendanceWidgetStore.preferences(context).edit().putString(messageKey, message).putLong(momentKey, moment).apply();
    }

    @Nullable
    static String recent(Context context, long moment) {
        SharedPreferences preferences = AttendanceWidgetStore.preferences(context);
        String message = preferences.getString(messageKey, null);
        if (message == null) return null;
        return moment - preferences.getLong(momentKey, 0) < shownFor ? message : null;
    }
}
