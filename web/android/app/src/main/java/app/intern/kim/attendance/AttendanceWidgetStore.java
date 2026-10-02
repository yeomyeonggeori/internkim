package app.intern.kim.attendance;

import android.content.Context;
import android.content.SharedPreferences;
import androidx.annotation.Nullable;
import java.util.UUID;

final class AttendanceWidgetStore {

    private static final String preferencesName = "attendance_widget";
    private static final String installKey = "widget.installID";
    private static final String tokenKey = "widget.token";
    private static final String tokenNameKey = "widget.tokenName";
    private static final String originKey = "widget.origin";
    private static final String workplaceKeyPrefix = "widget.workplace.";

    private AttendanceWidgetStore() {}

    static SharedPreferences preferences(Context context) {
        return context.getApplicationContext().getSharedPreferences(preferencesName, Context.MODE_PRIVATE);
    }

    static String installID(Context context) {
        SharedPreferences preferences = preferences(context);
        String held = preferences.getString(installKey, null);
        if (held != null) return held;
        String made = UUID.randomUUID().toString();
        preferences.edit().putString(installKey, made).apply();
        return made;
    }

    static String heldTokenName(Context context) {
        return preferences(context).getString(tokenNameKey, "");
    }

    @Nullable
    static Credential credential(Context context) {
        SharedPreferences preferences = preferences(context);
        String token = preferences.getString(tokenKey, null);
        String origin = preferences.getString(originKey, null);
        if (token == null || origin == null || origin.isEmpty()) return null;
        return new Credential(token, heldTokenName(context), origin);
    }

    static void keep(Context context, Credential credential) {
        preferences(context)
            .edit()
            .putString(tokenKey, credential.token)
            .putString(tokenNameKey, credential.tokenName)
            .putString(originKey, credential.origin)
            .apply();
    }

    static void forget(Context context) {
        preferences(context).edit().remove(tokenKey).remove(tokenNameKey).remove(originKey).apply();
    }

    @Nullable
    static String workplace(Context context, int widgetID) {
        return preferences(context).getString(workplaceKeyPrefix + widgetID, null);
    }

    static void keepWorkplace(Context context, int widgetID, String name) {
        preferences(context).edit().putString(workplaceKeyPrefix + widgetID, name).apply();
    }

    static void forgetWorkplace(Context context, int widgetID) {
        preferences(context).edit().remove(workplaceKeyPrefix + widgetID).apply();
    }

    static final class Credential {

        final String token;
        final String tokenName;
        final String origin;

        Credential(String token, String tokenName, String origin) {
            this.token = token;
            this.tokenName = tokenName;
            this.origin = origin;
        }
    }
}
