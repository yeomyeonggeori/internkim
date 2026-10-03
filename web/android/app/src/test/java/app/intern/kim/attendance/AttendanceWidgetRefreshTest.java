package app.intern.kim.attendance;

import static org.junit.Assert.assertEquals;

import org.junit.Test;

public class AttendanceWidgetRefreshTest {

    @Test
    public void theNextTickLandsWhereTheShownMinuteTurns() {
        assertEquals(30_000L, AttendanceWidgetRefresh.untilNextMinute(0));
        assertEquals(10_000L, AttendanceWidgetRefresh.untilNextMinute(20_000));
        assertEquals(50_000L, AttendanceWidgetRefresh.untilNextMinute(40_000));
    }

    @Test
    public void aTickRightOnTheTurnWaitsAWholeMinute() {
        assertEquals(60_000L, AttendanceWidgetRefresh.untilNextMinute(90_000));
    }
}
