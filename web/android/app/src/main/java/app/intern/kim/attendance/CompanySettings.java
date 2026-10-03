package app.intern.kim.attendance;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.TimeZone;

final class CompanySettings {

    final String timeZone;
    final List<WorkLocation> workLocations;

    CompanySettings(String timeZone, List<WorkLocation> workLocations) {
        this.timeZone = timeZone;
        this.workLocations = Collections.unmodifiableList(workLocations);
    }

    TimeZone companyTimeZone() {
        if (!Arrays.asList(TimeZone.getAvailableIDs()).contains(timeZone)) return TimeZone.getDefault();
        return TimeZone.getTimeZone(timeZone);
    }
}
