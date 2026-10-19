package app.intern.kim.attendance;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertTrue;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.TimeZone;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;
import org.junit.Test;

public class AttendanceTodayCasesTest {

    private static final String sharedCases = "../../tests/fixtures/attendance-today-cases.json";

    @Test
    public void theWidgetDrawsTheDayTheAttendanceCardDraws() throws IOException, JSONException {
        JSONArray cases = new JSONObject(new String(Files.readAllBytes(Paths.get(sharedCases)), StandardCharsets.UTF_8)).getJSONArray(
            "cases"
        );
        assertTrue(cases.length() > 0);

        for (int index = 0; index < cases.length(); index++) {
            JSONObject day = cases.getJSONObject(index);
            String name = day.getString("name");
            Long now = CompanyClock.moment(day.getString("now"));
            assertNotNull(name, now);
            TimeZone zone = TimeZone.getTimeZone(day.getString("timeZone"));
            assertEquals(name, day.getString("today"), CompanyClock.day(now, zone));
            assertEquals(name, day.getString("currentTime"), CompanyClock.time(now, zone));

            AttendanceToday computed = AttendanceToday.of(rowsOf(day.getJSONArray("rows")), day.getString("today"), now);
            JSONObject expected = day.getJSONObject("expected");

            assertEquals(name, expected.getInt("workedMinutes"), computed.workedMinutes());
            assertEquals(name, expected.getInt("elapsedMinutes"), computed.elapsedMinutes(now));
            assertEquals(name, expected.getBoolean("isWorking"), computed.isWorking());
            assertEquals(name, optional(expected, "location"), computed.location());
            assertEquals(name, optional(expected, "clockInTime"), computed.clockInTime());
            assertEquals(name, optional(expected, "clockOutTime"), computed.clockOutTime());

            JSONArray expectedBars = expected.getJSONArray("bars");
            List<AttendanceToday.Bar> bars = computed.bars(day.getString("currentTime"));
            assertEquals(name, expectedBars.length(), bars.size());
            for (int bar = 0; bar < bars.size(); bar++) {
                JSONObject expectedBar = expectedBars.getJSONObject(bar);
                assertEquals(name, optional(expectedBar, "location"), bars.get(bar).location);
                assertEquals(name, expectedBar.getDouble("widthPercent"), Math.round(bars.get(bar).widthPercent * 10000) / 10000.0, 0);
            }
        }
    }

    private static List<AttendanceRow> rowsOf(JSONArray listed) throws JSONException {
        List<AttendanceRow> rows = new ArrayList<>();
        for (int index = 0; index < listed.length(); index++) {
            JSONObject row = listed.getJSONObject(index);
            rows.add(
                new AttendanceRow(
                    "event-" + index,
                    row.getString("kind"),
                    row.getString("date"),
                    row.getString("time"),
                    row.getString("occurredAt"),
                    optional(row, "location")
                )
            );
        }
        return rows;
    }

    private static String optional(JSONObject object, String key) throws JSONException {
        return object.isNull(key) ? null : object.getString(key);
    }
}
