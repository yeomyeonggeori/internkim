package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;

final class AttendanceToday {

    private static final int minutesPerDay = 1440;
    private static final long millisPerMinute = 60_000;

    final List<Segment> segments;

    AttendanceToday(List<Segment> segments) {
        this.segments = Collections.unmodifiableList(segments);
    }

    static AttendanceToday nothing() {
        return new AttendanceToday(new ArrayList<>());
    }

    int workedMinutes() {
        int worked = 0;
        for (Segment segment : segments) worked += segment.workedMinutes;
        return worked;
    }

    @Nullable
    Segment activeSegment() {
        for (int index = segments.size() - 1; index >= 0; index--) {
            if (segments.get(index).isOpen) return segments.get(index);
        }
        return null;
    }

    boolean isWorking() {
        return activeSegment() != null;
    }

    @Nullable
    String location() {
        Segment active = activeSegment();
        return active == null ? null : active.location;
    }

    @Nullable
    String clockInTime() {
        return segments.isEmpty() ? null : segments.get(segments.size() - 1).clockInTime;
    }

    @Nullable
    String clockOutTime() {
        for (int index = segments.size() - 1; index >= 0; index--) {
            if (segments.get(index).clockOutTime != null) return segments.get(index).clockOutTime;
        }
        return null;
    }

    long elapsedMillis(long now) {
        long worked = workedMinutes() * millisPerMinute;
        Segment active = activeSegment();
        if (active == null) return worked;
        return worked + Math.max(0, now - active.clockInAt);
    }

    int elapsedMinutes(long now) {
        return (int) Math.round(elapsedMillis(now) / (double) millisPerMinute);
    }

    List<Bar> bars(String currentTime) {
        List<Bar> bars = new ArrayList<>();
        for (Segment segment : segments) {
            String end = segment.isOpen ? currentTime : (segment.endTime == null ? segment.startTime : segment.endTime);
            bars.add(new Bar(segment.location, dayWidthPercent(segment.startTime, end)));
        }
        return bars;
    }

    static AttendanceToday of(List<AttendanceRow> rows, String today, long now) {
        List<AttendanceRow> ordered = new ArrayList<>(rows);
        ordered.sort(Comparator.comparing(row -> row.occurredAt));
        List<Segment> segments = new ArrayList<>();
        AttendanceRow open = null;

        for (AttendanceRow row : ordered) {
            if (AttendanceRow.clockIn.equals(row.kind)) {
                if (open != null && !outlivedItsDay(open.occurredAt, row.occurredAt)) {
                    addIfPresent(segments, closedSegment(today, open, row, null));
                }
                open = row;
                continue;
            }
            if (open == null) continue;
            if (!outlivedItsDay(open.occurredAt, row.occurredAt)) {
                addIfPresent(segments, closedSegment(today, open, row, row.time));
            }
            open = null;
        }

        if (open != null) {
            Long startedAt = CompanyClock.moment(open.occurredAt);
            if (startedAt != null && minutesBetween(startedAt, now) <= minutesPerDay) {
                addIfPresent(segments, openSegment(today, open, startedAt));
            }
        }
        return new AttendanceToday(segments);
    }

    private static void addIfPresent(List<Segment> segments, @Nullable Segment segment) {
        if (segment != null) segments.add(segment);
    }

    @Nullable
    private static Segment closedSegment(String day, AttendanceRow clockIn, AttendanceRow end, @Nullable String clockOutTime) {
        Long startedAt = CompanyClock.moment(clockIn.occurredAt);
        if (startedAt == null) return null;
        if (day.compareTo(clockIn.date) < 0 || day.compareTo(end.date) > 0) return null;
        String startTime = day.equals(clockIn.date) ? clockIn.time : "00:00";
        String endTime = day.equals(end.date) ? end.time : "24:00";
        int worked = localMinutes(endTime) - localMinutes(startTime);
        if (worked <= 0) return null;
        return new Segment(clockIn.location, startTime, endTime, worked, false, clockIn.time, startedAt, clockOutTime);
    }

    @Nullable
    private static Segment openSegment(String day, AttendanceRow clockIn, long startedAt) {
        if (day.compareTo(clockIn.date) < 0) return null;
        String startTime = day.equals(clockIn.date) ? clockIn.time : "00:00";
        return new Segment(clockIn.location, startTime, null, 0, true, clockIn.time, startedAt, null);
    }

    private static boolean outlivedItsDay(String startedAt, String endedAt) {
        Long start = CompanyClock.moment(startedAt);
        Long end = CompanyClock.moment(endedAt);
        if (start == null || end == null) return false;
        return minutesBetween(start, end) > minutesPerDay;
    }

    private static int minutesBetween(long start, long end) {
        if (end <= start) return 0;
        return (int) Math.round((end - start) / (double) millisPerMinute);
    }

    static double dayWidthPercent(String startTime, String endTime) {
        int start = clamp(localMinutes(startTime));
        int end = clamp(Math.max(start, localMinutes(endTime)));
        return Math.min(100, Math.max(1, (end - start) / (double) minutesPerDay * 100));
    }

    private static int clamp(int minutes) {
        return Math.min(minutesPerDay, Math.max(0, minutes));
    }

    static int localMinutes(String localTime) {
        String[] parts = localTime.split(":");
        return wholeNumber(parts, 0) * 60 + wholeNumber(parts, 1);
    }

    private static int wholeNumber(String[] parts, int index) {
        if (index >= parts.length) return 0;
        try {
            return Integer.parseInt(parts[index]);
        } catch (NumberFormatException notANumber) {
            return 0;
        }
    }

    static final class Segment {

        @Nullable
        final String location;

        final String startTime;

        @Nullable
        final String endTime;

        final int workedMinutes;
        final boolean isOpen;
        final String clockInTime;
        final long clockInAt;

        @Nullable
        final String clockOutTime;

        Segment(
            @Nullable String location,
            String startTime,
            @Nullable String endTime,
            int workedMinutes,
            boolean isOpen,
            String clockInTime,
            long clockInAt,
            @Nullable String clockOutTime
        ) {
            this.location = location;
            this.startTime = startTime;
            this.endTime = endTime;
            this.workedMinutes = workedMinutes;
            this.isOpen = isOpen;
            this.clockInTime = clockInTime;
            this.clockInAt = clockInAt;
            this.clockOutTime = clockOutTime;
        }
    }

    static final class Bar {

        @Nullable
        final String location;

        final double widthPercent;

        Bar(@Nullable String location, double widthPercent) {
            this.location = location;
            this.widthPercent = widthPercent;
        }
    }
}
