<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { attendanceText } from '../text';
	import type { EmployeeLeaveRequest } from './employee-leave-types';
	import LeaveHistory from './leave-history.svelte';
	import LeaveRequestDialog from './leave-request-dialog.svelte';
	import LeaveTypeBalances from './leave-type-balances.svelte';

	const text = createPageText(attendanceText);
	let requestToEdit = $state<EmployeeLeaveRequest | undefined>();
</script>

<div class="mx-auto grid w-full max-w-5xl gap-4" data-testid="leave-history-view">
	<div>
		<h2 class="text-xl font-semibold">{text.leave.historyTab}</h2>
		<p class="mt-1 text-sm text-muted-foreground">{text.leave.historyDescription}</p>
	</div>
	<Card.Root class="gap-0">
		<Card.Content class="px-0">
			<LeaveTypeBalances />
			<LeaveHistory
				text={text.leave}
				onEdit={(request) => (requestToEdit = request)}
				onResubmit={(request) => (requestToEdit = request)}
			/>
		</Card.Content>
	</Card.Root>
	<LeaveRequestDialog
		request={requestToEdit}
		showTrigger={false}
		onClosed={() => (requestToEdit = undefined)}
	/>
</div>
