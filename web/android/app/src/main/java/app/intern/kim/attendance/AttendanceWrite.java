package app.intern.kim.attendance;

import androidx.annotation.Nullable;

final class AttendanceWrite {

    static final String removed = "removed";

    final String status;

    @Nullable
    final String eventID;

    @Nullable
    final Event event;

    AttendanceWrite(String status, @Nullable String eventID, @Nullable Event event) {
        this.status = status;
        this.eventID = eventID;
        this.event = event;
    }

    static final class Event {

        final String id;
        final String kind;
        final String occurredAt;

        @Nullable
        final String location;

        Event(String id, String kind, String occurredAt, @Nullable String location) {
            this.id = id;
            this.kind = kind;
            this.occurredAt = occurredAt;
            this.location = location;
        }
    }
}
