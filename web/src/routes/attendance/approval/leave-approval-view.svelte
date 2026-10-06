<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { attendanceText } from '../text';
	import LeaveApprovalCard from './leave-approval-card.svelte';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';
	import AttendanceLoadingSkeleton from '../attendance-loading-skeleton.svelte';
	import { Spinner } from '$lib/components/ui/spinner';

	const text = createPageText(attendanceText);
	const approval = getLeaveApprovalState();
</script>

<section class="mx-auto min-h-0 w-full max-w-6xl space-y-5" data-testid="leave-approval-view">
	<header class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{text.approval.title}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{text.approval.description}</p>
		</div>
		<Button
			variant="outline"
			size="sm"
			onclick={() => void approval.load()}
			disabled={approval.isLoading}
		>
			{#if approval.isLoading}<Spinner aria-label={text.loading} />{:else}<RefreshCwIcon />{/if}
			{text.approval.refresh}
		</Button>
	</header>

	{#if approval.errorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
			{approval.errorMessage}
		</p>
	{/if}
	{#if approval.mutationErrorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
			{approval.mutationErrorMessage}
		</p>
	{/if}

	{#if !approval.errorMessage || approval.inbox}<div class="divide-y rounded-lg border px-4" aria-busy={approval.isLoading}>
		{#if !approval.inbox}
			<AttendanceLoadingSkeleton kind="records" rowCount={3} />
		{:else if (approval.inbox?.pending.length ?? 0) === 0}
			<p class="rounded-lg border border-dashed py-12 text-center text-sm text-muted-foreground">
				{text.approval.pendingEmpty}
			</p>
		{:else}
			{#each approval.inbox?.pending ?? [] as request (request.id)}
				<LeaveApprovalCard {request} text={text.approval} />
			{/each}
		{/if}
	</div>{/if}
</section>
