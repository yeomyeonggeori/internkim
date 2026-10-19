package app.intern.kim.attendance;

import android.content.Context;
import java.util.Map;

public final class AttendanceWidgetPush {

    private static final String widgetKey = "widget";
    private static final String attendanceWidget = "attendance";

    private AttendanceWidgetPush() {}

    public static boolean isFor(Map<String, String> data) {
        return attendanceWidget.equals(data.get(widgetKey));
    }

    public static void redrawFromServer(Context context) {
        AttendanceWidgetCacheStore.distrustRows(context);
        AttendanceWidgetRefresh.redrawNow(context.getApplicationContext(), false);
    }
}
