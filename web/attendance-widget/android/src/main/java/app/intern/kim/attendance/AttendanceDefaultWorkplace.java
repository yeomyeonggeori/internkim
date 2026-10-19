package app.intern.kim.attendance;

import androidx.annotation.Nullable;
import java.util.ArrayList;
import java.util.List;

final class AttendanceDefaultWorkplace {

    @Nullable
    final WorkLocation location;

    @Nullable
    final String goneName;

    private AttendanceDefaultWorkplace(@Nullable WorkLocation location, @Nullable String goneName) {
        this.location = location;
        this.goneName = goneName;
    }

    static AttendanceDefaultWorkplace of(@Nullable String configured, List<WorkLocation> locations) {
        if (configured == null || configured.isEmpty()) {
            return new AttendanceDefaultWorkplace(locations.isEmpty() ? null : locations.get(0), null);
        }
        for (WorkLocation location : locations) {
            if (location.name.equals(configured)) return new AttendanceDefaultWorkplace(location, null);
        }
        return new AttendanceDefaultWorkplace(null, configured);
    }

    static List<WorkLocation> others(@Nullable WorkLocation chosen, List<WorkLocation> locations) {
        List<WorkLocation> others = new ArrayList<>();
        for (WorkLocation location : locations) {
            if (chosen == null || !location.name.equals(chosen.name)) others.add(location);
        }
        return others;
    }

    boolean needsChoice() {
        return goneName != null;
    }
}
