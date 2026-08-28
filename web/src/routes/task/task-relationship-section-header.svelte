<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TaskRelationshipProgressBadge from './task-relationship-progress-badge.svelte';
	import type { TaskChildProgress } from './task-relationships';

	type Props = {
		title: string;
		addLabel: string;
		progressLabel?: string;
		progress?: TaskChildProgress;
		actionable?: boolean;
		disabled?: boolean;
		onAdd?: () => void;
	};

	let {
		title,
		addLabel,
		progressLabel = title,
		progress,
		actionable = false,
		disabled = false,
		onAdd
	}: Props = $props();
</script>

{#snippet content()}
	<div class="flex min-w-0 items-center gap-2">
		<h4 class="text-xs font-semibold tracking-wide text-muted-foreground">{title}</h4>
		{#if progress}
			<TaskRelationshipProgressBadge
				completed={progress.completed}
				total={progress.total}
				percent={progress.percent}
				label={progressLabel}
			/>
		{/if}
	</div>
	{#if actionable}
		<span class="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
			<PlusIcon data-icon="inline-start" />
			{addLabel}
		</span>
	{/if}
{/snippet}

{#if actionable}
	<Button
		variant="ghost"
		class="h-9 w-full justify-between px-2 text-left"
		{disabled}
		aria-label={`${title} ${addLabel}`}
		onclick={onAdd}
	>
		{@render content()}
	</Button>
{:else}
	<div class="flex min-h-9 items-center justify-between gap-3 px-2">
		{@render content()}
	</div>
{/if}
