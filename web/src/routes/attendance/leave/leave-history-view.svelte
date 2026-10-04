<script lang="ts">
	import { onMount } from 'svelte';
	import { getEmployeeLeaveState } from './employee-leave-state.svelte';
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import LeaveHistory from './leave-history.svelte';
	import LeaveTypeBalances from './leave-type-balances.svelte';

	const text = createPageText(attendanceText);
	const employeeLeave = getEmployeeLeaveState();
	onMount(() => { void employeeLeave.ensureLoaded(); });
</script>

<div class="mx-auto grid w-full max-w-5xl gap-4" data-testid="leave-history-view">
	<div>
		<h2 class="text-xl font-semibold">{text.leave.historyTab}</h2>
		<p class="mt-1 text-sm text-muted-foreground">{text.leave.historyDescription}</p>
	</div>
	<Card.Root class="gap-0">
		<Card.Content class="px-0">
			<LeaveTypeBalances />
			<LeaveHistory text={text.leave} />
		</Card.Content>
	</Card.Root>
</div>
