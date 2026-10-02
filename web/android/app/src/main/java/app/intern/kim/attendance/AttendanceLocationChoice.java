package app.intern.kim.attendance;

import android.content.Context;
import androidx.annotation.Nullable;

final class AttendanceLocationChoice {

    static final long shownFor = 20 * 1000L;

    private static final String openedKey = "widget.locations.opened";

    private AttendanceLocationChoice() {}

    static void open(Context context, long moment) {
        AttendanceWidgetStore.preferences(context).edit().putLong(openedKey, moment).apply();
    }

    static void close(Context context) {
        AttendanceWidgetStore.preferences(context).edit().remove(openedKey).apply();
    }

    @Nullable
    static Long closes(Context context, long moment) {
        long opened = AttendanceWidgetStore.preferences(context).getLong(openedKey, 0);
        if (opened <= 0) return null;
        long closing = opened + shownFor;
        return closing > moment ? closing : null;
    }
}
