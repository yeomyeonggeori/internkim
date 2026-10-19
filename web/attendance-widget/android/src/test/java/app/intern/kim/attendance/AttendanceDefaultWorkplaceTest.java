package app.intern.kim.attendance;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertNull;
import static org.junit.Assert.assertSame;
import static org.junit.Assert.assertTrue;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import org.junit.Test;

public class AttendanceDefaultWorkplaceTest {

    private final WorkLocation office = new WorkLocation("Office", "#112233");
    private final WorkLocation home = new WorkLocation("Home", null);
    private final List<WorkLocation> both = Arrays.asList(office, home);

    @Test
    public void nothingConfiguredKeepsTakingTheFirstWorkplace() {
        assertSame(office, AttendanceDefaultWorkplace.of(null, both).location);
    }

    @Test
    public void anEmptyConfiguredNameIsTreatedAsNothingConfigured() {
        assertSame(office, AttendanceDefaultWorkplace.of("", both).location);
    }

    @Test
    public void aCompanyWithNoWorkplaceChoosesNothing() {
        AttendanceDefaultWorkplace choice = AttendanceDefaultWorkplace.of(null, Collections.emptyList());
        assertNull(choice.location);
        assertNull(choice.goneName);
    }

    @Test
    public void theConfiguredWorkplaceWinsOverTheFirstOne() {
        assertSame(home, AttendanceDefaultWorkplace.of("Home", both).location);
    }

    @Test
    public void aWorkplaceTheCompanyNoLongerHasAsksForAChoiceInsteadOfClockingIn() {
        AttendanceDefaultWorkplace choice = AttendanceDefaultWorkplace.of("Warehouse", both);
        assertEquals("Warehouse", choice.goneName);
        assertNull(choice.location);
        assertTrue(choice.needsChoice());
    }

    @Test
    public void theOtherWorkplacesLeaveOutWhicheverIsDefault() {
        assertEquals(Collections.singletonList(office), AttendanceDefaultWorkplace.others(home, both));
    }

    @Test
    public void theOtherWorkplacesKeepEveryOneWhenNoneIsDefault() {
        assertEquals(both, AttendanceDefaultWorkplace.others(null, both));
    }
}
