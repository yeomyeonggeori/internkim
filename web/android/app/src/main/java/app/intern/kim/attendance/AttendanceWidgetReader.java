package app.intern.kim.attendance;

import android.content.Context;
import java.util.List;
import java.util.TimeZone;

final class AttendanceWidgetReader {

    private AttendanceWidgetReader() {}

    static AttendanceWidgetEntry read(Context context, long now) {
        try {
            AttendanceAPI api = AttendanceAPI.held(context);
            String origin = api.credential.origin;
            AttendanceWidgetCache cache = AttendanceWidgetCacheStore.held(context, origin);
            boolean drawsTap = AttendanceWidgetCache.drawsTapFromCache(cache.tappedAt, now);
            boolean isStale = false;

            CompanySettings settings;
            if (cache.settings != null && AttendanceWidgetCache.settingsAreFresh(cache.settingsFetchedAt, now)) {
                settings = cache.settings;
            } else if (cache.settings != null && drawsTap) {
                settings = cache.settings;
                isStale = true;
            } else {
                CompanySettings fetched = api.settings();
                AttendanceWidgetCacheStore.amend(context, origin, held -> {
                    held.settings = fetched;
                    held.settingsFetchedAt = now;
                });
                settings = fetched;
            }
            TimeZone zone = settings.companyTimeZone();

            List<AttendanceRow> rows = AttendanceWidgetCache.rowsShown(
                cache.rows,
                cache.pending,
                cache.rowsTrustedUntil,
                drawsTap && cache.rowsListedAt != null,
                now
            );
            if (rows != null) {
                isStale = isStale || cache.rowsTrustedUntil == null || now >= cache.rowsTrustedUntil;
            } else {
                List<AttendanceRow> listed = api.sinceYesterday(now, zone);
                AttendanceWidgetCacheStore.amend(context, origin, held -> {
                    held.rows = listed;
                    held.rowsListedAt = now;
                });
                rows = listed;
            }

            AttendanceToday today = AttendanceToday.of(rows, CompanyClock.day(now, zone), now);
            return new AttendanceWidgetEntry(now, today, settings.workLocations, zone, null, isStale);
        } catch (AttendanceAPI.Failure failure) {
            return AttendanceWidgetEntry.failed(now, failure.getMessage());
        }
    }
}
