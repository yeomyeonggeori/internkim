package app.intern.kim.attendance;

import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertTrue;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import org.junit.Test;

public class AttendanceWidgetPushTest {

    @Test
    public void theRefreshThePlaneSendsIsRecognised() {
        Map<String, String> data = new HashMap<>();
        data.put("widget", "attendance");
        data.put("clock", "clock_in");
        assertTrue(AttendanceWidgetPush.isFor(data));
    }

    @Test
    public void aNotificationForAPersonIsLeftToTheNotificationPoster() {
        Map<String, String> data = new HashMap<>();
        data.put("title", "박예시 출근");
        data.put("body", "사무실 · 09:02");
        assertFalse(AttendanceWidgetPush.isFor(data));
        assertFalse(AttendanceWidgetPush.isFor(Collections.emptyMap()));
    }
}
