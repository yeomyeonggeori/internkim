package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.List;
import java.util.TimeZone;

final class AttendanceWidgetCache {

    static final long settingsFreshFor = 6 * 60 * 60 * 1000L;
    static final long rowsTrustedFor = 120 * 1000L;
    static final long tapDrawnFromCacheFor = 5 * 1000L;

    final String origin;

    @Nullable
    CompanySettings settings;

    @Nullable
    Long settingsFetchedAt;

    List<AttendanceRow> rows;

    @Nullable
    Long rowsTrustedUntil;

    @Nullable
    AttendanceRow pending;

    @Nullable
    Long rowsListedAt;

    @Nullable
    Long tappedAt;

    AttendanceWidgetCache(String origin) {
        this.origin = origin;
        this.rows = new ArrayList<>();
    }

    static AttendanceWidgetCache resolve(@Nullable AttendanceWidgetCache decoded, String origin) {
        if (decoded == null || !decoded.origin.equals(origin)) return new AttendanceWidgetCache(origin);
        return decoded;
    }

    static boolean drawsTapFromCache(@Nullable Long tappedAt, long now) {
        return tappedAt != null && now - tappedAt < tapDrawnFromCacheFor;
    }

    static boolean settingsAreFresh(@Nullable Long fetchedAt, long now) {
        return fetchedAt != null && now - fetchedAt < settingsFreshFor;
    }

    @Nullable
    static List<AttendanceRow> rowsShown(
        List<AttendanceRow> cachedRows,
        @Nullable AttendanceRow pending,
        @Nullable Long rowsTrustedUntil,
        boolean drawsTap,
        long now
    ) {
        boolean isTrusted = rowsTrustedUntil != null && now < rowsTrustedUntil;
        if (!isTrusted && !drawsTap) return null;
        List<AttendanceRow> shown = new ArrayList<>(cachedRows);
        if (pending != null) shown.add(pending);
        return shown;
    }

    void settle(@Nullable AttendanceRow pressed, AttendanceWrite written, @Nullable AttendanceRow added, long now) {
        letGoOf(pressed);
        if (AttendanceWrite.removed.equals(written.status) && written.eventID != null) {
            List<AttendanceRow> kept = new ArrayList<>();
            for (AttendanceRow row : rows) {
                if (!row.eventID.equals(written.eventID)) kept.add(row);
            }
            rows = kept;
            rowsTrustedUntil = now + rowsTrustedFor;
        } else if (added != null) {
            rows = appending(added, rows);
            rowsTrustedUntil = now + rowsTrustedFor;
        } else {
            rowsTrustedUntil = null;
        }
    }

    void refuse(@Nullable AttendanceRow pressed) {
        letGoOf(pressed);
        rowsTrustedUntil = null;
    }

    private void letGoOf(@Nullable AttendanceRow pressed) {
        if (pending == null || pressed == null) return;
        if (!pending.kind.equals(pressed.kind) || !pending.occurredAt.equals(pressed.occurredAt)) return;
        pending = null;
    }

    static List<AttendanceRow> appending(AttendanceRow row, List<AttendanceRow> rows) {
        List<AttendanceRow> appended = new ArrayList<>();
        for (AttendanceRow held : rows) {
            if (!held.eventID.equals(row.eventID)) appended.add(held);
        }
        appended.add(row);
        return appended;
    }

    static AttendanceRow optimisticRow(String kind, @Nullable String location, long now, TimeZone timeZone) {
        return new AttendanceRow(
            AttendanceRow.pendingID,
            kind,
            CompanyClock.day(now, timeZone),
            CompanyClock.time(now, timeZone),
            CompanyClock.internetString(now),
            AttendanceRow.clockIn.equals(kind) ? location : null
        );
    }

    @Nullable
    static AttendanceRow row(AttendanceWrite.Event event, TimeZone timeZone) {
        Long occurred = CompanyClock.moment(event.occurredAt);
        if (occurred == null) return null;
        return new AttendanceRow(
            event.id,
            event.kind,
            CompanyClock.day(occurred, timeZone),
            CompanyClock.time(occurred, timeZone),
            event.occurredAt,
            event.location
        );
    }
}
