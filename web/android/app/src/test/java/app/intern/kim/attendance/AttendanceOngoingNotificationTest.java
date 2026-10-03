package app.intern.kim.attendance;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import java.util.Arrays;
import java.util.Collections;
import org.junit.Test;

public class AttendanceOngoingNotificationTest {

    private final AttendanceRow clockIn = new AttendanceRow("e1", "clock_in", "2026-09-22", "09:00", "2026-09-22T00:00:00Z", "재택");
    private final AttendanceRow clockOut = new AttendanceRow("e2", "clock_out", "2026-09-22", "18:00", "2026-09-22T09:00:00Z", null);
    private final long clockInAt = CompanyClock.moment(clockIn.occurredAt);
    private final long sameDay = CompanyClock.moment("2026-09-22T03:00:00Z");

    @Test
    public void aMemberAtWorkSeesTheNotification() {
        AttendanceToday working = AttendanceToday.of(Collections.singletonList(clockIn), "2026-09-22", sameDay);
        assertTrue(AttendanceOngoingNotification.shows(working, null));
    }

    @Test
    public void aMemberWhoClockedOutSeesNone() {
        AttendanceToday done = AttendanceToday.of(Arrays.asList(clockIn, clockOut), "2026-09-22", sameDay);
        assertFalse(AttendanceOngoingNotification.shows(done, null));
    }

    @Test
    public void aNotificationSwipedAwayStaysAwayUntilTheNextClockIn() {
        AttendanceToday working = AttendanceToday.of(Collections.singletonList(clockIn), "2026-09-22", sameDay);
        assertFalse(AttendanceOngoingNotification.shows(working, clockInAt));
        assertTrue(AttendanceOngoingNotification.shows(working, clockInAt - 60_000));
    }
}
