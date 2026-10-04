<script lang="ts">
	import DeferredSection from '$lib/components/deferred-section.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import { getAttendanceState } from '../attendance-context.svelte';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import AttendanceTeamDashboard from './attendance-team-dashboard.svelte';
	import TeamPersonMonth from './team-person-month.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	let showMonthly = $state(false);
	let selectedMember = $state<AttendanceTeamPage['members'][number] | null>(null);

	function openMonthly(member?: AttendanceTeamPage['members'][number]): void {
		selectedMember = member ?? null;
		showMonthly = true;
		if (!member) void attendance.load();
	}
</script>

<div class="flex min-h-0 min-w-0 flex-col">
	{#if myAttendanceToday.summary && !myAttendanceToday.summary.isAdmin && !myAttendanceToday.summary.teamViewVisibleToAll}
		<p class="text-sm text-muted-foreground">{text.teamBlocked}</p>
	{:else if showMonthly}
		<Button variant="ghost" size="sm" class="mb-3 self-start" onclick={() => (showMonthly = false)}>{text.teamTodayOpen}</Button>
		{#if selectedMember}
			<TeamPersonMonth member={selectedMember} />
		{:else if attendance.summary}
			<DeferredSection>
				<TeamStatusGrid />
				{#snippet placeholder()}
					<AttendanceLoadingSkeleton rowCount={8} />
				{/snippet}
			</DeferredSection>
		{:else if attendance.errorMessage}
			<p role="alert" class="text-sm text-destructive">{attendance.errorMessage}</p>
			<Button variant="outline" size="sm" onclick={() => attendance.load()}>{text.refresh}</Button>
		{:else}
			<AttendanceLoadingSkeleton rowCount={8} />
		{/if}
	{:else}
		<AttendanceTeamDashboard onOpenMonthly={openMonthly} />
	{/if}
</div>
