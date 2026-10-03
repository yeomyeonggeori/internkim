package app.intern.kim.attendance;

import androidx.annotation.Nullable;

final class AttendanceRow {

    static final String clockIn = "clock_in";
    static final String clockOut = "clock_out";
    static final String pendingID = "pending";

    final String eventID;
    final String kind;
    final String date;
    final String time;
    final String occurredAt;

    @Nullable
    final String location;

    AttendanceRow(String eventID, String kind, String date, String time, String occurredAt, @Nullable String location) {
        this.eventID = eventID;
        this.kind = kind;
        this.date = date;
        this.time = time;
        this.occurredAt = occurredAt;
        this.location = location;
    }
}
