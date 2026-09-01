<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { attendanceText } from '../text';
	import AttendanceApprovalCard from './attendance-approval-card.svelte';
	import { getAttendanceApprovalState } from './attendance-approval-state.svelte';
	import LeaveApprovalCard from './leave-approval-card.svelte';
	import { getLeaveApprovalState } from './leave-approval-state.svelte';

	type ApprovalTab = 'pending' | 'recent' | 'attendance';

	const text = createPageText(attendanceText);
	const approval = getLeaveApprovalState();
	const attendanceApproval = getAttendanceApprovalState();
	let selectedTab = $state<ApprovalTab>('pending');
</script>

<section class="mx-auto min-h-0 w-full max-w-6xl space-y-5" data-testid="leave-approval-view">
	<header class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{text.approval.title}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{text.approval.description}</p>
		</div>
		<Button
			variant="outline"
			size="sm"
			onclick={() => void Promise.all([approval.load(), attendanceApproval.load()])}
			disabled={approval.isLoading}
		>
			<RefreshCwIcon class={approval.isLoading ? 'animate-spin' : ''} />
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

	<Tabs.Root bind:value={selectedTab} class="gap-4">
		<Tabs.List variant="line">
			<Tabs.Trigger value="pending" class="gap-2 px-3">
				{text.approval.pendingTab}
				<Badge variant="secondary">{approval.inbox?.pendingCount ?? 0}</Badge>
			</Tabs.Trigger>
			<Tabs.Trigger value="recent" class="px-3">{text.approval.recentTab}</Tabs.Trigger>
			<Tabs.Trigger value="attendance" class="gap-2 px-3" data-testid="attendance-approval-tab">
				{text.approval.attendanceTab}
				<Badge variant="secondary">{attendanceApproval.requests.length}</Badge>
			</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="pending" class="space-y-3">
			{#if approval.isLoading && !approval.inbox}
				<p class="py-12 text-center text-sm text-muted-foreground">{text.approval.loading}</p>
			{:else if (approval.inbox?.pending.length ?? 0) === 0}
				<p class="rounded-lg border border-dashed py-12 text-center text-sm text-muted-foreground">
					{text.approval.pendingEmpty}
				</p>
			{:else}
				{#each approval.inbox?.pending ?? [] as request (request.id)}
					<LeaveApprovalCard {request} text={text.approval} />
				{/each}
			{/if}
		</Tabs.Content>

		<Tabs.Content value="recent" class="space-y-3">
			{#if (approval.inbox?.recentChanges.length ?? 0) === 0}
				<p class="rounded-lg border border-dashed py-12 text-center text-sm text-muted-foreground">
					{text.approval.recentEmpty}
				</p>
			{:else}
				{#each approval.inbox?.recentChanges ?? [] as change (`${change.request.id}-${change.changedAt}`)}
					<LeaveApprovalCard request={change.request} text={text.approval} {change} />
				{/each}
			{/if}
		</Tabs.Content>

		<Tabs.Content value="attendance" class="space-y-3">
			{#if attendanceApproval.errorMessage}
				<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{attendanceApproval.errorMessage}
				</p>
			{/if}
			{#if attendanceApproval.mutationErrorMessage}
				<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
					{attendanceApproval.mutationErrorMessage}
				</p>
			{/if}
			{#if attendanceApproval.requests.length === 0}
				<p class="rounded-lg border border-dashed py-12 text-center text-sm text-muted-foreground">
					{text.approval.attendanceEmpty}
				</p>
			{:else}
				{#each attendanceApproval.requests as request (request.id)}
					<AttendanceApprovalCard {request} {text} />
				{/each}
			{/if}
		</Tabs.Content>
	</Tabs.Root>
</section>
