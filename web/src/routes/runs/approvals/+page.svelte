<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Empty from '$lib/components/ui/empty';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import HourglassIcon from '@lucide/svelte/icons/hourglass';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { page } from '$app/state';
	import { taskListPathOf, taskRunDetailPathOf } from '$lib/app-shell';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { onMount } from 'svelte';
	import ApprovalDecision from '../approval-decision.svelte';
	import { fetchPendingApprovals, type PendingApproval } from '../runs-api';
	import { formatTaskTimestamp } from '../runs-view';
	import { tasksText } from '../text';
	import { isRunsAccessDenied } from '../runs-read-error';

	const text = createPageText(tasksText);
	let approvals = $state<PendingApproval[] | undefined>(undefined);
	let loadError = $state('');
	let isLoading = $state(false);
	let loadGeneration = 0;

	async function load() {
		const generation = ++loadGeneration;
		isLoading = true;
		loadError = '';
		try {
			const response = await fetchPendingApprovals();
			if (generation === loadGeneration) approvals = response;
		} catch (error) {
			if (generation !== loadGeneration) return;
			if (isRunsAccessDenied(error)) approvals = undefined;
			loadError = text.approvalsLoadError;
		} finally {
			if (generation === loadGeneration) isLoading = false;
		}
	}

	function forgetDecidedApproval(taskRunID: string) {
		approvals = (approvals ?? []).filter((approval) => approval.taskRun.taskRunID !== taskRunID);
	}

	onMount(load);
</script>

<svelte:head>
	<title>{text.approvalsPageTitle}</title>
</svelte:head>

<main class="flex min-h-full w-full self-start flex-col gap-5 px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h1 class="text-xl font-semibold">{text.approvalsTitle}</h1>
		<div class="flex items-center gap-2">
			<Button href={taskListPathOf(page.url.pathname)} variant="ghost" size="sm">
				<ArrowLeftIcon data-icon="inline-start" />
				{text.backToList}
			</Button>
			<Button variant="outline" size="sm" disabled={isLoading} onclick={load}>
				<RefreshCwIcon data-icon="inline-start" />
				{text.refresh}
			</Button>
		</div>
	</div>

	{#if loadError}
		<Card.Root size="sm" class="border-destructive/30">
			<Card.Content class="text-sm text-destructive">{loadError}</Card.Content>
		</Card.Root>
		{/if}
		{#if approvals === undefined && !loadError}
			<section role="status" aria-label={text.approvalsTitle} aria-busy="true" class="grid gap-4" data-testid="approvals-loading-skeleton">
				{#each [0, 1] as card (card)}<div aria-hidden="true" class="grid gap-4 rounded-xl border p-6"><div class="flex flex-wrap gap-3"><Skeleton class="h-5 w-24 rounded-full" /><Skeleton class="h-4 w-28" /><Skeleton class="h-4 w-28" /></div><Skeleton class="h-5 w-3/4" /><Skeleton class="h-4 w-full" /><Skeleton class="h-4 w-2/3" /><div class="flex gap-2"><Skeleton class="h-9 w-24" /><Skeleton class="h-9 w-24" /></div><Skeleton class="h-4 w-20" /></div>{/each}
			</section>
		{:else if approvals?.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<HourglassIcon />
				</Empty.Media>
				<Empty.Title>{text.approvalsEmpty}</Empty.Title>
			</Empty.Header>
		</Empty.Root>
		{:else if approvals}
		<section class="flex flex-col gap-4">
			{#each approvals as approval (approval.taskRun.taskRunID)}
				<Card.Root>
					<Card.Header class="gap-3">
						<div class="flex min-w-0 flex-wrap items-center gap-2">
							<Badge variant="outline">
								<HourglassIcon />
								{text.statusWaitingApproval}
							</Badge>
							{#if approval.taskRun.requesterDisplayName || approval.taskRun.requesterPersonID}
								<span class="text-xs text-muted-foreground">
								{text.requesterLabel}: {displayPersonName(approval.taskRun.requesterDisplayName || approval.taskRun.requesterPersonID)}
								</span>
							{/if}
							<span class="text-xs text-muted-foreground">{formatTaskTimestamp(approval.taskRun.updatedAt)}</span>
						</div>
						<Card.Title class="text-base">{approval.taskRun.prompt || '—'}</Card.Title>
					</Card.Header>
					<Card.Content>
						<ApprovalDecision
							{approval}
							{text}
							onDecided={() => forgetDecidedApproval(approval.taskRun.taskRunID)}
						/>
					</Card.Content>
					<Card.Footer>
						<Button href={taskRunDetailPathOf(page.url.pathname, approval.taskRun.taskRunID)} variant="ghost" size="sm">
							{text.openLedger}
						</Button>
					</Card.Footer>
				</Card.Root>
			{/each}
		</section>
	{/if}
</main>
