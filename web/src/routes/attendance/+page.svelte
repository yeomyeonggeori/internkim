<script lang="ts">
	import { getAttendanceState } from './attendance-context.svelte';
	import TeamView from './team/team-view.svelte';
	import { attendanceText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);

	const isTeamBlocked = $derived(
		!!attendance.summary && !attendance.summary.isAdmin && attendance.summary.teamViewBlocked
	);
</script>

{#if isTeamBlocked}
	<p class="text-sm text-muted-foreground">{text.teamBlocked}</p>
{:else}
	<TeamView />
{/if}
