<script lang="ts">
	import PersonWorkStandard from './person-work-standard.svelte';
	import { ToolRefused } from '$lib/public-api-call';
	import { Button } from '$lib/components/ui/button';
	import { onMount } from 'svelte';
	import { companyDateOf } from '$lib/company-time';
	import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
	import { supabaseAttendancePersonSummary } from '$lib/attendance/supabase-attendance';
	import type { AttendanceTeamPage } from '$lib/attendance/team-page';
	import { AttendanceState, setAttendanceState, type AttendanceSummary } from '../attendance-context.svelte';
	import { createAttendanceServerClock } from '../shared/attendance-server-clock';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import { getAttendanceTeamState } from './attendance-team-state.svelte';
	import TeamStatusGrid from './team-status-grid.svelte';
	import { WorkStatusState, setWorkStatusState } from '../work-status/work-status-state.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';

	let { member }: { member: AttendanceTeamPage['members'][number] } = $props();
	const teamState = getAttendanceTeamState();
	const text = createPageText(attendanceText);
	const recordState = new AttendanceState(text.loadFailed, '', async () => {
		await Promise.all([teamState.refresh(), myAttendanceToday.refresh()]);
	});
	setAttendanceState(recordState);
	const standardState = new WorkStatusState();
	setWorkStatusState(standardState);
	recordState.load = async () => { await load(); };
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
		if (summary?.month !== nextMonth) {
			summary = null;
			recordState.summary = null;
			recordState.serverClock = null;
		}
		loading = true;
		error = '';
		try {
			const answer = await supabaseAttendancePersonSummary(nextMonth, {
				memberID: member.memberID, email: member.email, displayName: member.name
			}, requester);
			if (disposed || request !== sequence || requesterScope !== currentScope()) return;
			summary = answer;
			standardState.payload = null;
			standardState.monthPayload = null;
			recordState.summary = answer;
			recordState.selectedMonth = answer.month;
			recordState.serverClock = createAttendanceServerClock(answer.serverTime, performance.now());
		} catch (reason) {
			if (disposed || request !== sequence || requesterScope !== currentScope()) return;
			if (reason instanceof ToolRefused && [401, 403].includes(reason.status)) {
				summary = null;
				recordState.summary = null;
				recordState.serverClock = null;
			}
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
		return () => { disposed = true; sequence += 1; recordState.dispose(); standardState.dispose(); };
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
{/if}
{#if summary}
	<div class:opacity-60={loading} aria-busy={loading}>
		{#if member.memberID === summary.currentMemberID}{#key summary.month}<PersonWorkStandard {summary} />{/key}{/if}
		<TeamStatusGrid {summary} onSelectMonth={(nextMonth) => load(nextMonth)} initialMemberName={member.name} backLabel={member.name} />
	</div>
{:else if !error}
	<div aria-busy="true">
		{#if member.memberID === myAttendanceToday.summary?.currentMemberID}<Button variant="ghost" size="sm" disabled>{text.workStatus.standard}</Button>{/if}
		<AttendanceLoadingSkeleton kind="month" rowCount={8} />
	</div>
{/if}
