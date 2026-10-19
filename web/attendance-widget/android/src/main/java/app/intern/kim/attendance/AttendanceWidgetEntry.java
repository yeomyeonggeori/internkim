package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.TimeZone;

final class AttendanceWidgetEntry {

    final long date;
    final AttendanceToday today;
    final List<WorkLocation> locations;
    final TimeZone timeZone;

    @Nullable
    final String failure;

    final boolean isStale;

    AttendanceWidgetEntry(
        long date,
        AttendanceToday today,
        List<WorkLocation> locations,
        TimeZone timeZone,
        @Nullable String failure,
        boolean isStale
    ) {
        this.date = date;
        this.today = today;
        this.locations = Collections.unmodifiableList(locations);
        this.timeZone = timeZone;
        this.failure = failure;
        this.isStale = isStale;
    }

    static AttendanceWidgetEntry failed(long date, String failure) {
        return new AttendanceWidgetEntry(date, AttendanceToday.nothing(), new ArrayList<>(), TimeZone.getDefault(), failure, false);
    }

    String currentTime() {
        return CompanyClock.time(date, timeZone);
    }
}
