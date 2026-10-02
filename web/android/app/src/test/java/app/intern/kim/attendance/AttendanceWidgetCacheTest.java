package app.intern.kim.attendance;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertFalse;
import static org.junit.Assert.assertNotNull;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertTrue;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.TimeZone;
import org.json.JSONException;
import org.json.JSONObject;
import org.junit.Test;

public class AttendanceWidgetCacheTest {

    private static final long now = 1_789_600_000_000L;
    private static final TimeZone seoul = TimeZone.getTimeZone("Asia/Seoul");
    private static final String alpha = "https://alpha.example.com";

    private final AttendanceRow sampleRow = new AttendanceRow("e1", "clock_in", "2026-09-23", "09:00", "2026-09-23T00:00:00Z", null);
    private final AttendanceRow pendingRow = new AttendanceRow(
        "pending",
        "clock_out",
        "2026-09-23",
        "18:00",
        "2026-09-23T09:00:00Z",
        null
    );

    @Test
    public void optimisticRowCarriesTheTappedKindAndLocation() {
        AttendanceRow row = AttendanceWidgetCache.optimisticRow("clock_in", "재택", now, seoul);

        assertEquals("pending", row.eventID);
        assertEquals("clock_in", row.kind);
        assertEquals("재택", row.location);
        assertEquals(CompanyClock.day(now, seoul), row.date);
        assertEquals(CompanyClock.time(now, seoul), row.time);
        assertEquals(Long.valueOf(now), CompanyClock.moment(row.occurredAt));
    }

    @Test
    public void optimisticRowDropsLocationForClockOut() {
        assertNull(AttendanceWidgetCache.optimisticRow("clock_out", "재택", now, seoul).location);
    }

    @Test
    public void eventToRowConvertsUsingTheGivenTimeZoneAcrossMidnightUTC() {
        AttendanceRow row = AttendanceWidgetCache.row(
            new AttendanceWrite.Event("evt-1", "clock_in", "2026-09-23T15:30:00Z", "사무실"),
            seoul
        );

        assertNotNull(row);
        assertEquals("evt-1", row.eventID);
        assertEquals("2026-09-24", row.date);
        assertEquals("00:30", row.time);
        assertEquals("2026-09-23T15:30:00Z", row.occurredAt);
        assertEquals("사무실", row.location);
    }

    @Test
    public void aMomentWithFractionalSecondsAndAnOffsetIsRead() {
        assertEquals(CompanyClock.moment("2026-09-23T15:30:00Z"), CompanyClock.moment("2026-09-24T00:30:00.123456+09:00"));
    }

    @Test
    public void eventToRowFailsWhenOccurredAtCannotBeParsed() {
        assertNull(AttendanceWidgetCache.row(new AttendanceWrite.Event("evt-1", "clock_in", "not-a-date", null), seoul));
    }

    @Test
    public void rowsShownServeCachedRowsAndThePendingTapWhileTrusted() {
        List<AttendanceRow> shown = AttendanceWidgetCache.rowsShown(
            Collections.singletonList(sampleRow),
            pendingRow,
            now + 60_000,
            false,
            now
        );

        assertNotNull(shown);
        assertEquals(2, shown.size());
        assertEquals("e1", shown.get(0).eventID);
        assertEquals("pending", shown.get(1).eventID);
    }

    @Test
    public void rowsShownSignalARefetchOnceTrustExpiresEvenWithAPendingTap() {
        assertNull(AttendanceWidgetCache.rowsShown(Collections.singletonList(sampleRow), pendingRow, now - 1000, false, now));
    }

    @Test
    public void rowsShownSignalARefetchWithNoTrustWindow() {
        assertNull(AttendanceWidgetCache.rowsShown(Collections.singletonList(sampleRow), null, null, false, now));
    }

    @Test
    public void rowsShownServeHeldRowsForATapAfterTrustExpires() {
        List<AttendanceRow> shown = AttendanceWidgetCache.rowsShown(Collections.singletonList(sampleRow), null, now - 1000, true, now);

        assertNotNull(shown);
        assertEquals(1, shown.size());
        assertEquals("e1", shown.get(0).eventID);
    }

    @Test
    public void aTapIsDrawnFromCacheOnlyRightAfterIt() {
        assertTrue(AttendanceWidgetCache.drawsTapFromCache(now - 1000, now));
        assertFalse(AttendanceWidgetCache.drawsTapFromCache(now - AttendanceWidgetCache.tapDrawnFromCacheFor, now));
        assertFalse(AttendanceWidgetCache.drawsTapFromCache(null, now));
    }

    @Test
    public void aCacheWrittenBeforeTapsWereRecordedStillDecodes() throws JSONException {
        AttendanceWidgetCache decoded = AttendanceJSON.cache(new JSONObject("{\"origin\":\"https://alpha.example.com\",\"rows\":[]}"));

        assertNull(decoded.tappedAt);
        assertNull(decoded.rowsListedAt);
    }

    @Test
    public void aCacheSurvivesBeingWrittenAndReadBack() throws JSONException {
        AttendanceWidgetCache cache = new AttendanceWidgetCache(alpha);
        cache.rows = Collections.singletonList(sampleRow);
        cache.pending = pendingRow;
        cache.rowsTrustedUntil = now;
        cache.settings = new CompanySettings("Asia/Seoul", Collections.singletonList(new WorkLocation("사무실", "#112233")));

        AttendanceWidgetCache read = AttendanceJSON.cache(new JSONObject(AttendanceJSON.of(cache).toString()));

        assertEquals("e1", read.rows.get(0).eventID);
        assertEquals("pending", read.pending.eventID);
        assertEquals(Long.valueOf(now), read.rowsTrustedUntil);
        assertEquals("#112233", read.settings.workLocations.get(0).color);
        assertNull(read.tappedAt);
    }

    @Test
    public void appendingReplacesARowTheListAlreadyCarries() {
        List<AttendanceRow> rows = AttendanceWidgetCache.appending(sampleRow, Collections.singletonList(sampleRow));

        assertEquals(1, rows.size());
        assertEquals("e1", rows.get(0).eventID);
    }

    @Test
    public void anEarlierPressSettlingLeavesALaterPressPending() {
        AttendanceRow earlier = AttendanceWidgetCache.optimisticRow("clock_out", null, now, seoul);
        AttendanceRow later = AttendanceWidgetCache.optimisticRow("clock_in", "사무실", now + 1000, seoul);
        AttendanceWidgetCache cache = new AttendanceWidgetCache(alpha);
        cache.pending = later;

        cache.settle(earlier, new AttendanceWrite("added", "e2", null), null, now);

        assertEquals("clock_in", cache.pending.kind);
    }

    @Test
    public void aTakenBackPressDropsTheRowItRemoved() {
        AttendanceWidgetCache cache = new AttendanceWidgetCache(alpha);
        cache.rows = new ArrayList<>(Collections.singletonList(sampleRow));
        cache.pending = pendingRow;

        cache.settle(pendingRow, new AttendanceWrite("removed", sampleRow.eventID, null), null, now);

        assertTrue(cache.rows.isEmpty());
        assertNull(cache.pending);
        assertEquals(Long.valueOf(now + AttendanceWidgetCache.rowsTrustedFor), cache.rowsTrustedUntil);
    }

    @Test
    public void settingsAreFreshWithinSixHours() {
        assertTrue(AttendanceWidgetCache.settingsAreFresh(now - AttendanceWidgetCache.settingsFreshFor + 1000, now));
    }

    @Test
    public void settingsAreStaleAtSixHours() {
        assertFalse(AttendanceWidgetCache.settingsAreFresh(now - AttendanceWidgetCache.settingsFreshFor, now));
    }

    @Test
    public void settingsAreStaleWhenNeverFetched() {
        assertFalse(AttendanceWidgetCache.settingsAreFresh(null, now));
    }

    @Test
    public void anOriginMismatchIsTreatedAsEmpty() {
        AttendanceWidgetCache held = new AttendanceWidgetCache(alpha);
        held.rows = Collections.singletonList(sampleRow);
        held.rowsTrustedUntil = now;
        held.pending = sampleRow;

        AttendanceWidgetCache resolved = AttendanceWidgetCache.resolve(held, "https://beta.example.com");

        assertEquals("https://beta.example.com", resolved.origin);
        assertTrue(resolved.rows.isEmpty());
        assertNull(resolved.settings);
        assertNull(resolved.rowsTrustedUntil);
        assertNull(resolved.pending);
    }

    @Test
    public void aMatchingOriginIsKept() {
        AttendanceWidgetCache held = new AttendanceWidgetCache(alpha);

        assertEquals(alpha, AttendanceWidgetCache.resolve(held, alpha).origin);
    }

    @Test
    public void noDecodedCacheIsTreatedAsEmpty() {
        AttendanceWidgetCache resolved = AttendanceWidgetCache.resolve(null, alpha);

        assertTrue(resolved.rows.isEmpty());
        assertEquals(alpha, resolved.origin);
    }
}
