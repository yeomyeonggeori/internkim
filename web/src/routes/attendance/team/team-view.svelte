<script lang="ts">
 import { createPageText } from '$lib/i18n/page-text.svelte';
 import { attendanceText } from '../text';
 import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
 import AttendanceTeamDashboard from './attendance-team-dashboard.svelte';
 const text = createPageText(attendanceText);
</script>
<div class="flex min-h-0 min-w-0 flex-col">
 {#if myAttendanceToday.summary && !myAttendanceToday.summary.isAdmin && !myAttendanceToday.summary.teamViewVisibleToAll}
  <p class="text-sm text-muted-foreground">{text.teamBlocked}</p>
 {:else}
  <AttendanceTeamDashboard />
 {/if}
</div>
