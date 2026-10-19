package app.intern.kim.attendance;

import android.content.Context;

public final class AttendanceWidgetHandoff {

    private AttendanceWidgetHandoff() {}

    public static String installID(Context context) {
        return AttendanceWidgetStore.installID(context);
    }

    public static String heldTokenName(Context context) {
        return AttendanceWidgetStore.heldTokenName(context);
    }

    public static void supply(Context context, String token, String tokenName, String origin) {
        AttendanceWidgetStore.keep(context, new AttendanceWidgetStore.Credential(token, tokenName, origin));
        AttendanceWidgetProvider.redrawSoon(context);
    }

    public static void forget(Context context) {
        AttendanceWidgetStore.forget(context);
        AttendanceWidgetProvider.redrawSoon(context);
    }
}
