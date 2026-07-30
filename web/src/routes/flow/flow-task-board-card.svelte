<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';
	import * as Card from '$lib/components/ui/card';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import { buildFlowTaskBoardCardDisplay } from './flow-task-board-card-model';
	import FlowTaskDateRange from './flow-task-date-range.svelte';
	import FlowTaskPersonChip from './flow-task-person-chip.svelte';
	import { sizeBadgeClass } from './flow-style';
	import { flowBusinessBadgeStyle } from './flow-business-color';
	import type { FlowTask } from './flow-types';
	import type { Snippet } from 'svelte';

	type Props = {
		task: FlowTask;
		businessFallback: string;
		openTask: (task: FlowTask) => void;
		isPending?: boolean;
		isReadOnly?: boolean;
		isDraggable?: boolean;
		onTaskDragStart?: (event: DragEvent, task: FlowTask) => void;
		onTaskDragEnd?: (event: DragEvent, task: FlowTask) => void;
		onTaskDragOver?: (event: DragEvent, task: FlowTask) => void;
		onTaskDrop?: (event: DragEvent, task: FlowTask) => void;
		ownerChip?: Snippet;
		isInteractive?: boolean;
		memberEmail?: (memberID: string) => string;
		isOverduePlan?: boolean;
		businessColor?: (business: string) => string;
	};

	let {
		task,
		businessFallback,
		openTask,
		isPending = false,
		isReadOnly = false,
		isDraggable = true,
		onTaskDragStart,
		onTaskDragEnd,
		onTaskDragOver,
		onTaskDrop,
		ownerChip,
		isInteractive = true,
		memberEmail = () => '',
		isOverduePlan = false,
		businessColor = () => '#64748b'
	}: Props = $props();

	let canDrag = $derived(isInteractive && isDraggable && !isPending && !isReadOnly);
	let isDragging = $state(false);
	let cardClass = $derived([
		'flow-task-board-card gap-0 rounded-md border border-border/80 bg-card p-0',
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

	let display = $derived(buildFlowTaskBoardCardDisplay(task, businessFallback));
	let primaryParticipantName = $derived(display.participantNames[0] ?? '');
	let primaryParticipantID = $derived(display.participantIDs[0] ?? '');
	let additionalParticipantCount = $derived(Math.max(display.participantNames.length - 1, 0));

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
		currentTarget.classList.add('flow-task-board-card-dragging');
		const { horizontalOffset, verticalOffset } = dragImageOffset(event, currentTarget);
		event.dataTransfer?.setDragImage(currentTarget, horizontalOffset, verticalOffset);
	}

	function handleTaskDragEnd(event: DragEvent): void {
		isDragging = false;
		const currentTarget = event.currentTarget;
		if (currentTarget instanceof HTMLElement) currentTarget.classList.remove('flow-task-board-card-dragging');
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
		data-flow-board-card={task.id}
		data-flow-board-pending={isPending ? 'true' : 'false'}
		data-flow-board-dragging={isDragging ? 'true' : 'false'}
	onclick={openCurrentTask}
	onkeydown={(event) => {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			openCurrentTask();
		}
	}}
	ondragstart={handleTaskDragStart}
	ondragend={handleTaskDragEnd}
	ondragover={(event) => onTaskDragOver?.(event, task)}
	ondrop={(event) => onTaskDrop?.(event, task)}
>
	<div class="space-y-1 px-3 py-2">
		<div class="flex min-w-0 items-center justify-between gap-2">
			<div class="flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
				{#if ownerChip}
					{@render ownerChip()}
				{:else}
					<FlowTaskPersonChip name={display.ownerName} email={memberEmail(task.ownerID)} seed={task.ownerID || display.ownerName} image={personProfileImagePath(task.ownerID)} />
				{/if}
				{#if primaryParticipantName}
					<span class="inline-flex min-w-0 max-w-24 items-center gap-1.5">
						<PersonAvatar name={primaryParticipantName} email={memberEmail(primaryParticipantID)} seed={primaryParticipantID || primaryParticipantName} image={personProfileImagePath(primaryParticipantID)} class="size-3.5 ring-1 ring-border/60" />
						<span class="truncate">{primaryParticipantName}</span>
					</span>
				{/if}
				{#if additionalParticipantCount > 0}
					<span class="shrink-0 rounded-full bg-muted px-1.5 text-[11px] leading-5 text-muted-foreground">
						+{additionalParticipantCount}
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
						class="h-5 max-w-24 rounded-md border-transparent px-1.5 py-0 text-[11px] font-medium shadow-none"
						style={flowBusinessBadgeStyle(businessColor(task.business))}
					>
						{display.businessLabel}
					</Badge>
				{/if}
				{#each display.metadataLabels as label}
					<Badge variant="outline" class="h-5 max-w-24 rounded-md border-border/70 bg-muted/30 px-1.5 py-0 text-[11px] font-normal text-muted-foreground shadow-none">{label}</Badge>
				{/each}
				{#if task.startDate || task.endDate}
					<Badge
						variant={isOverduePlan ? 'destructive' : 'secondary'}
						class={cn(
							'h-5 max-w-full rounded-md px-1.5 py-0 text-[11px] font-medium shadow-none',
							!isOverduePlan && 'bg-muted text-foreground/75'
						)}
					>
						<FlowTaskDateRange startDate={task.startDate} endDate={task.endDate} />
					</Badge>
				{/if}
			</div>
		{/if}
	</div>
</Card.Root>

<style>
	:global(.flow-task-board-card-dragging) {
		background: hsl(var(--card)) !important;
		border-color: hsl(var(--border)) !important;
		box-shadow: 0 8px 16px hsl(var(--foreground) / 0.12) !important;
		opacity: 0.96;
	}
</style>
