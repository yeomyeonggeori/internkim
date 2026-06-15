<!-- Flow 업무 보드 카드 표시를 담당합니다. -->
<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import { buildFlowTaskBoardCardDisplay } from './flow-task-board-card-model';
	import { sizeBadgeClass } from './flow-style';
	import type { FlowTask } from './flow-types';

	type Props = {
		task: FlowTask;
		openTask: (task: FlowTask) => void;
	};

	let { task, openTask }: Props = $props();

	const cardClass = [
		'cursor-pointer gap-1.5 rounded-md border bg-card p-2.5 shadow-xs transition',
		'hover:border-primary/40 hover:shadow-sm'
	].join(' ');

	let display = $derived(buildFlowTaskBoardCardDisplay(task));
</script>

<Card.Root
	class={cardClass}
	role="button"
	tabindex={0}
	onclick={() => openTask(task)}
	onkeydown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			openTask(task);
		}
	}}
>
	<div class="flex min-w-0 flex-wrap items-center gap-1">
		<div class="mr-1 truncate text-xs text-muted-foreground">{display.ownerName}</div>
		{#each display.participantNames as name}
			<Badge variant="outline" class="max-w-20 truncate px-1.5 py-0 text-xs">{name}</Badge>
		{/each}
	</div>

	<div class="flex items-start justify-between gap-2">
		<div class="min-w-0">
			<div class="line-clamp-2 text-sm font-semibold leading-5 text-card-foreground">{task.content}</div>
		</div>
		<Badge class={`h-6 shrink-0 px-2 text-xs ${sizeBadgeClass(task.size)}`}>{task.size}</Badge>
	</div>

	{#if task.goal}
		<div class="line-clamp-1 text-xs leading-5 text-muted-foreground">{task.goal}</div>
	{/if}

	{#if display.metadataLabels.length > 0}
		<div class="flex flex-wrap items-center gap-1 pt-0.5">
			{#each display.metadataLabels as label}
				<Badge variant="outline" class="max-w-24 truncate px-1.5 py-0 text-xs">{label}</Badge>
			{/each}
		</div>
	{/if}

	{#if display.dateLabel}
		<div class="pt-0.5">
			<Badge variant="secondary" class="max-w-full truncate px-1.5 py-0 text-xs">{display.dateLabel}</Badge>
		</div>
	{/if}
</Card.Root>
