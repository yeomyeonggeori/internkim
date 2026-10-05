<script lang="ts">
	import PersonWorkStandard from './person-work-standard.svelte';
	import { onMount } from 'svelte';
	import { companyDateOf } from '$lib/company-time';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { supabaseAttendancePersonSummary } from '$lib/attendance/supabase-attendance';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import type { AttendanceSummary } from '../attendance-context.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import { getAttendanceTeamState } from './attendance-team-state.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';

	let { member }: { member: AttendanceTeamPage['members'][number] } = $props();
	const teamState = getAttendanceTeamState();
	let month = $state('');
	let summary = $state<AttendanceSummary | null>(null);
	let error = $state('');
	let loading = $state(false);
	let disposed = false;
	let sequence = 0;
	let seenRevision = teamState.revision;

	async function load(nextMonth = month): Promise<void> {
		const requester = myAttendanceToday.summary;
		if (disposed || !requester || !nextMonth) return;
		const request = ++sequence;
        const requesterScope = [requester.currentMemberID, requester.currentUserEmail, requester.isAdmin, requester.teamViewVisibleToAll].join("|");
        const currentScope = () => { const current = myAttendanceToday.summary; return current ? [current.currentMemberID, current.currentUserEmail, current.isAdmin, current.teamViewVisibleToAll].join("|") : ""; };
		month = nextMonth;
		if (summary?.month !== nextMonth) summary = null;
		loading = true;
		error = '';
		try {
			const answer = await supabaseAttendancePersonSummary(nextMonth, {
				memberID: member.memberID, email: member.email, displayName: member.name
			}, requester);
			if (disposed || request !== sequence || requesterScope !== currentScope()) return;
			summary = answer;
		} catch (reason) {
			if (disposed || request !== sequence || requesterScope !== currentScope()) return;
			summary = null;
			error = reason instanceof Error ? reason.message : String(reason);
		} finally {
			if (!disposed && request === sequence) loading = false;
		}
	}

	onMount(() => {
		const requester = myAttendanceToday.summary;
		if (requester) {
			month = companyDateOf(new Date(requester.serverTime ?? Date.now()), requester.timeZone).slice(0, 7);
			void load();
		}
		return () => { disposed = true; sequence += 1; };
	});

	$effect(() => {
		const revision = teamState.revision;
		if (revision === seenRevision) return;
		seenRevision = revision;
		void load();
	});
</script>

{#if error}
	<p role="alert" class="text-sm text-destructive">{error}</p>
	<button type="button" class="text-sm underline" onclick={() => load()}>Retry</button>
{:else if summary}
	<div class:opacity-60={loading}>
		{#if member.memberID === summary.currentMemberID}{#key summary.month}<PersonWorkStandard {summary} />{/key}{/if}
		<TeamStatusGrid {summary} onSelectMonth={(nextMonth) => load(nextMonth)} initialMemberName={member.name} />
	</div>
{:else}
	<AttendanceLoadingSkeleton rowCount={8} />
{/if}
