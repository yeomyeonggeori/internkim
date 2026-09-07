<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import PersonAvatarStack from '$lib/components/person-avatar-stack.svelte';
	import { displayPersonName } from '$lib/person-name.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';
	import * as Card from '$lib/components/ui/card';
	import { buildTaskBoardCardDisplay } from './task-board-card-model';
	import TaskChildProgress from './task-child-progress.svelte';
	import TaskDateRange from './task-date-range.svelte';
	import DefinitionBadge from '$lib/components/definition-badge.svelte';
	import { sizeBadgeClass } from './task-style';
	import { taskDefinitionBadgeStyle } from './task-definition-colors';
	import type { TaskChildProgress as ChildProgress } from './task-relationships';
	import type { Task } from './task-types';
	import type { Snippet } from 'svelte';

	type Props = {
		task: Task;
		etcLabel: string;
		openTask: (task: Task) => void;
		isPending?: boolean;
		isReadOnly?: boolean;
		isDraggable?: boolean;
		onTaskDragStart?: (event: DragEvent, task: Task) => void;
		onTaskDragEnd?: (event: DragEvent, task: Task) => void;
		primaryParticipantChip?: Snippet;
		isInteractive?: boolean;
		memberEmail?: (memberID: string) => string;
		isOverduePlan?: boolean;
		businessColor?: (business: string | null) => string;
		taskTypeColor?: (type: string | null) => string;
		childProgress?: ChildProgress;
		childProgressLabel?: string;
	};

	let {
		task,
		etcLabel,
		openTask,
		isPending = false,
		isReadOnly = false,
		isDraggable = true,
		onTaskDragStart,
		onTaskDragEnd,
		primaryParticipantChip,
		isInteractive = true,
		memberEmail = () => '',
		isOverduePlan = false,
		businessColor = () => '#64748b',
		taskTypeColor = () => '#64748b',
		childProgress,
		childProgressLabel = '{completed} / {total}'
	}: Props = $props();

	let canDrag = $derived(isInteractive && isDraggable && !isPending && !isReadOnly);
	let isDragging = $state(false);
	let cardClass = $derived([
		'task-board-card gap-0 rounded-md border border-border/80 bg-card p-0',
		'outline-none ring-0 shadow-xs transition-[background-color,box-shadow,opacity]',
		isPending
			? 'cursor-progress opacity-60 ring-1 ring-primary/20'
			: !isInteractive
				? 'cursor-default'
			: canDrag
				? 'cursor-grab hover:bg-muted/30 hover:shadow-sm active:cursor-grabbing active:bg-muted/40'
				: 'cursor-pointer hover:bg-muted/30 hover:shadow-sm',
		isDragging ? 'bg-card opacity-95 shadow-md' : '',
		isReadOnly ? 'bg-muted/20' : ''
	].join(' '));

	let display = $derived(buildTaskBoardCardDisplay(task, etcLabel));
	let primaryParticipantName = $derived(display.participantNames[0] ?? '');
	let primaryParticipantID = $derived(display.participantIDs[0] ?? '');
	let participants = $derived(
		display.participantNames.map((name, index) => ({
			name,
			seed: display.participantIDs[index] || name,
			email: memberEmail(display.participantIDs[index] ?? '')
		}))
	);
	let participantNameList = $derived(participants.map((person) => displayPersonName(person.name)).join(', '));

	function openCurrentTask(): void {
		if (isPending || !isInteractive) return;
		openTask(task);
	}

	function handleTaskDragStart(event: DragEvent): void {
		onTaskDragStart?.(event, task);
		if (event.defaultPrevented) return;
		isDragging = true;
		const currentTarget = event.currentTarget;
		if (!(currentTarget instanceof HTMLElement)) return;
		currentTarget.classList.add('task-board-card-dragging');
		const { horizontalOffset, verticalOffset } = dragImageOffset(event, currentTarget);
		event.dataTransfer?.setDragImage(currentTarget, horizontalOffset, verticalOffset);
	}

	function handleTaskDragEnd(event: DragEvent): void {
		isDragging = false;
		const currentTarget = event.currentTarget;
		if (currentTarget instanceof HTMLElement) currentTarget.classList.remove('task-board-card-dragging');
		onTaskDragEnd?.(event, task);
	}

	function dragImageOffset(event: DragEvent, element: HTMLElement): { horizontalOffset: number; verticalOffset: number } {
		const bounds = element.getBoundingClientRect();
		if (event.clientX > 0 || event.clientY > 0) {
			return {
				horizontalOffset: clampNumber(event.clientX - bounds.left, 0, bounds.width),
				verticalOffset: clampNumber(event.clientY - bounds.top, 0, bounds.height)
			};
		}
		return {
			horizontalOffset: bounds.width / 2,
			verticalOffset: Math.min(bounds.height / 2, 36)
		};
	}

	function clampNumber(value: number, minimum: number, maximum: number): number {
		return Math.min(Math.max(value, minimum), maximum);
	}
</script>

<Card.Root
		class={cardClass}
		role={isInteractive ? 'button' : undefined}
		tabindex={isInteractive && !isPending ? 0 : undefined}
		draggable={canDrag}
		aria-disabled={isPending}
		data-task-board-card={task.id}
		data-task-board-pending={isPending ? 'true' : 'false'}
		data-task-board-dragging={isDragging ? 'true' : 'false'}
	onclick={openCurrentTask}
	onkeydown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			openCurrentTask();
		}
	}}
	ondragstart={handleTaskDragStart}
	ondragend={handleTaskDragEnd}
>
	<div class="space-y-1 px-3 py-2">
		<div class="flex min-w-0 items-center justify-between gap-2">
			<div class="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
				{#if primaryParticipantChip}
					{@render primaryParticipantChip()}
				{:else if participants.length > 1}
					<PersonAvatarStack
						people={participants}
						max={3}
						class="-space-x-1"
						avatarClass="size-3.5 ring-1 ring-card"
						label={participantNameList}
					/>
				{:else if primaryParticipantName}
					<span class="inline-flex min-w-0 max-w-24 items-center gap-1.5">
						<PersonAvatar name={primaryParticipantName} email={memberEmail(primaryParticipantID)} seed={primaryParticipantID || primaryParticipantName} class="size-3.5 ring-1 ring-border/60" />
						<span class="truncate">{displayPersonName(primaryParticipantName)}</span>
					</span>
				{/if}
			</div>
			<Badge class={`h-[18px] min-w-7 shrink-0 justify-center px-1.5 text-[11px] font-semibold leading-none ${sizeBadgeClass(task.size)}`}>{task.size}</Badge>
		</div>

		<div class="line-clamp-2 text-sm font-semibold leading-5 text-card-foreground">
			{task.content}
		</div>

		{#if display.businessLabel || display.metadataLabels.length > 0 || task.startDate || task.endDate}
			<div class="flex flex-wrap items-center gap-1.5">
				{#if display.businessLabel}
					<Badge
						class="h-5 max-w-24 border-transparent px-1.5 py-0 text-[11px] font-medium shadow-none"
						style={taskDefinitionBadgeStyle(businessColor(task.business))}
					>
						{display.businessLabel}
					</Badge>
				{/if}
				{#each display.metadataLabels as label}
					<DefinitionBadge {label} color={taskTypeColor(label)} class="max-w-24" />
				{/each}
				{#if task.startDate || task.endDate}
					<Badge
						variant={isOverduePlan ? 'destructive' : 'secondary'}
						class={cn(
							'h-5 max-w-full rounded-md px-1.5 py-0 text-[11px] font-medium shadow-none',
							!isOverduePlan && 'bg-muted text-foreground/75'
						)}
					>
						<TaskDateRange startDate={task.startDate} endDate={task.endDate} />
					</Badge>
				{/if}
			</div>
		{/if}

		{#if childProgress}
			<TaskChildProgress
				progress={childProgress}
				label={childProgressLabel
					.replace('{completed}', String(childProgress.completed))
					.replace('{total}', String(childProgress.total))}
			/>
		{/if}
	</div>
</Card.Root>

<style>
	:global(.task-board-card-dragging) {
		background: hsl(var(--card)) !important;
		border-color: hsl(var(--border)) !important;
		box-shadow: 0 8px 16px hsl(var(--foreground) / 0.12) !important;
		opacity: 0.96;
	}
</style>
