package app.intern.kim.attendance;

import android.content.Context;
import androidx.annotation.Nullable;
import java.util.TimeZone;

final class AttendanceClockPress {

    private AttendanceClockPress() {}

    static void press(Context context, String kind, @Nullable String location) {
        AttendanceLocationChoice.close(context);

        AttendanceAPI api;
        try {
            api = AttendanceAPI.held(context);
        } catch (AttendanceAPI.Failure noKey) {
            AttendanceRefusal.keep(context, noKey.getMessage(), System.currentTimeMillis());
            AttendanceWidgetRefresh.redrawNow(context, false);
            return;
        }

        String origin = api.credential.origin;
        CompanySettings settings = AttendanceWidgetCacheStore.held(context, origin).settings;
        TimeZone zone = settings == null ? null : settings.companyTimeZone();
        long tapped = System.currentTimeMillis();
        AttendanceRow optimistic = zone == null ? null : AttendanceWidgetCache.optimisticRow(kind, location, tapped, zone);

        if (optimistic != null) {
            AttendanceWidgetCacheStore.amend(context, origin, cache -> {
                cache.pending = optimistic;
                cache.tappedAt = tapped;
                cache.rowsTrustedUntil = tapped + AttendanceWidgetCache.rowsTrustedFor;
            });
            AttendanceWidgetRefresh.redrawNow(context, false);
        }

        save(context, api, kind, location, zone, optimistic);
    }

    private static void save(
        Context context,
        AttendanceAPI api,
        String kind,
        @Nullable String location,
        @Nullable TimeZone zone,
        @Nullable AttendanceRow pressed
    ) {
        String origin = api.credential.origin;
        try {
            AttendanceWrite written = api.clock(kind, location);
            AttendanceRow added = zone == null || written.event == null ? null : AttendanceWidgetCache.row(written.event, zone);
            AttendanceWidgetCacheStore.amend(context, origin, cache ->
                cache.settle(pressed, written, added, System.currentTimeMillis())
            );
        } catch (AttendanceAPI.Failure refused) {
            AttendanceWidgetCacheStore.amend(context, origin, cache -> cache.refuse(pressed));
            AttendanceRefusal.keep(context, refused.getMessage(), System.currentTimeMillis());
        }
        AttendanceWidgetRefresh.redrawNow(context, false);
    }
}
