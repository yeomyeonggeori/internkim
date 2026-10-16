<script lang="ts">
	import { buildTaskBoard } from '../../routes/task/task-board-model';
	import TaskBoardCard from '../../routes/task/task-board-card.svelte';
	import { taskStatusIcon } from '../../routes/task/task-status-style';
	import { taskText } from '../../routes/task/text';
	import ActivityAvatar from './activity-avatar.svelte';
	import { activityTasks, maskedSegments, type CompanyActivity } from './activity';
	import { dataRoomCompanyText } from './company-text';

	let { activity, locale }: { activity: CompanyActivity; locale: 'ko' | 'en' } = $props();

	const cardsPerColumn = 3;
	const neutralColor = '#a1a1aa';
	const text = $derived(dataRoomCompanyText[locale].activity);
	const statusText = $derived(taskText[locale].status);
	const tasks = $derived(activityTasks(activity).map((task) => ({ ...task, size: '' })));
	const columns = $derived(
		buildTaskBoard(tasks, { weekStartISO: activity.weekStart, weekEndISO: activity.weekEnd, weekPosition: 'current' })
	);

	function neutral(): string {
		return neutralColor;
	}

	function statusLabel(status: string): string {
		return statusText[status as keyof typeof statusText] ?? status;
	}

	function ignoreOpen(): void {}
</script>

<div class="grid gap-3 lg:grid-cols-5">
		{#each columns as column (column.status)}
			{@const StatusIcon = taskStatusIcon(column.status)}
			{@const total = activity.board.counts[column.status] ?? column.tasks.length}
			<section class="bg-muted/30 flex min-w-0 flex-col overflow-hidden rounded-lg border" aria-label={statusLabel(column.status)}>
				<header class="bg-card flex h-11 items-center gap-2 border-b px-3">
					<StatusIcon class="text-muted-foreground size-4 shrink-0" aria-hidden="true" />
					<h4 class="truncate text-sm font-semibold">{statusLabel(column.status)}</h4>
					<span class="bg-muted text-muted-foreground inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 text-xs font-medium tabular-nums">{total}</span>
				</header>
				<div class="grid content-start gap-2 p-2.5">
					{#each column.tasks.slice(0, cardsPerColumn) as task, index (task.id)}
						<div class={index > 0 ? 'max-lg:hidden' : ''}>
						<TaskBoardCard
							{task}
							etcLabel=""
							openTask={ignoreOpen}
							isInteractive={false}
							isDraggable={false}
							businessColor={neutral}
							taskTypeColor={neutral}
						>
							{#snippet primaryParticipantChip()}
								<span class="inline-flex min-w-0 items-center gap-1.5">
									<span class="flex -space-x-1">
										{#each task.participantIDs.slice(0, 3) as person (person)}
											<ActivityAvatar person={activity.people[person]} seed={person} class="ring-card size-4 ring-1" />
										{/each}
									</span>
									<span class="truncate">{task.participantNames[0] ?? ''}</span>
								</span>
							{/snippet}
							{#snippet content()}
								{#each maskedSegments(task.content) as segment, index (index)}
									{#if segment.isHidden}
										<span class="bg-foreground/40 inline-block h-[0.7em] rounded-[2px]" style:width={`${segment.text.length * 0.8}em`} role="img" aria-label={text.hidden}></span>
									{:else}{segment.text}{/if}
								{/each}
							{/snippet}
						</TaskBoardCard>
						</div>
					{:else}
						<p class="text-muted-foreground py-6 text-center text-xs">—</p>
					{/each}
					{#if total > 1}<p class="text-muted-foreground px-1 pt-1 text-xs tabular-nums lg:hidden">{text.moreTasks(total - 1)}</p>{/if}
					{#if total > cardsPerColumn}
						<p class="text-muted-foreground px-1 pt-1 text-xs tabular-nums max-lg:hidden">{text.moreTasks(total - Math.min(cardsPerColumn, column.tasks.length))}</p>
					{/if}
				</div>
			</section>
		{/each}
</div>
