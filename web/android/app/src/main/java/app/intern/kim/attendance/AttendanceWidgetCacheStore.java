package app.intern.kim.attendance;

import android.content.Context;
import android.util.Log;
import org.json.JSONException;
import org.json.JSONObject;

final class AttendanceWidgetCacheStore {

    private static final String logTag = "AttendanceWidget";
    private static final String cacheKey = "widget.attendanceCache";
    private static final Object writing = new Object();

    interface Change {
        void apply(AttendanceWidgetCache cache);
    }

    private AttendanceWidgetCacheStore() {}

    static AttendanceWidgetCache held(Context context, String origin) {
        String encoded = AttendanceWidgetStore.preferences(context).getString(cacheKey, null);
        if (encoded == null) return new AttendanceWidgetCache(origin);
        try {
            return AttendanceWidgetCache.resolve(AttendanceJSON.cache(new JSONObject(encoded)), origin);
        } catch (JSONException unreadable) {
            Log.w(logTag, "the held attendance cache could not be read and is drawn as empty", unreadable);
            return new AttendanceWidgetCache(origin);
        }
    }

    static void amend(Context context, String origin, Change change) {
        synchronized (writing) {
            AttendanceWidgetCache cache = held(context, origin);
            change.apply(cache);
            try {
                AttendanceWidgetStore.preferences(context)
                    .edit()
                    .putString(cacheKey, AttendanceJSON.of(cache).toString())
                    .apply();
            } catch (JSONException unwritable) {
                Log.w(logTag, "the attendance cache could not be written", unwritable);
            }
        }
    }

    static void distrustRows(Context context) {
        AttendanceWidgetStore.Credential credential = AttendanceWidgetStore.credential(context);
        if (credential == null) return;
        amend(context, credential.origin, cache -> {
            cache.rowsTrustedUntil = null;
            cache.rowsListedAt = null;
            cache.tappedAt = null;
        });
    }

    static void markTap(Context context, long moment) {
        AttendanceWidgetStore.Credential credential = AttendanceWidgetStore.credential(context);
        if (credential == null) return;
        amend(context, credential.origin, cache -> cache.tappedAt = moment);
    }
}
