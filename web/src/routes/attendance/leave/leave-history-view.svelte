<script lang="ts">
	import LeaveRequestDialog from './leave-request-dialog.svelte';
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


<section class="mx-auto grid w-full max-w-5xl gap-5" data-testid="leave-history-view">
 <header class="flex items-center justify-between gap-3"><h1 class="text-2xl font-semibold tracking-tight">{text.navigation.mine}</h1><LeaveRequestDialog /></header>
 {#if employeeLeave.errorMessage}<p role="alert" class="rounded-md border border-destructive/40 p-3 text-sm text-destructive">{employeeLeave.errorMessage}</p>{:else}
  <LeaveTypeBalances />
  <section class="space-y-3"><h2 class="text-sm font-semibold">{text.leave.historyTitle}</h2><Card.Root class="gap-0 py-0"><Card.Content class="px-0"><LeaveHistory text={text.leave} /></Card.Content></Card.Root></section>
 {/if}
</section>
