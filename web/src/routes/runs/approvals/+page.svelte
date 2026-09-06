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

	const text = createPageText(tasksText);
	let approvals = $state<PendingApproval[] | undefined>(undefined);
	let loadError = $state('');
	let isLoading = $state(false);

	async function load() {
		isLoading = true;
		loadError = '';
		try {
			approvals = await fetchPendingApprovals();
		} catch {
			loadError = text.approvalsLoadError;
		} finally {
			isLoading = false;
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

<main class="flex min-h-[calc(100svh-48px)] w-full self-start flex-col gap-5 px-4 py-4 sm:px-6 sm:py-5 lg:px-8">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="grid gap-1">
			<h1 class="text-xl font-semibold">{text.approvalsTitle}</h1>
			<p class="text-sm text-muted-foreground">{text.approvalsDescription}</p>
		</div>
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
	{:else if approvals === undefined}
		<section class="flex flex-col gap-3">
			<Skeleton class="h-40 w-full" />
			<Skeleton class="h-40 w-full" />
		</section>
	{:else if approvals.length === 0}
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					<HourglassIcon />
				</Empty.Media>
				<Empty.Title>{text.approvalsEmpty}</Empty.Title>
			</Empty.Header>
		</Empty.Root>
	{:else}
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
