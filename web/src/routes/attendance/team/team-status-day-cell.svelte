<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip/index.js';
	import { mergeProps } from 'bits-ui';
	import DurationText from '../shared/duration-text.svelte';
	import WorkSegmentSummary from '../shared/work-segment-summary.svelte';
	import type { TeamStatusPersonDay } from './team-status-table-model';

	type Props = {
		day: TeamStatusPersonDay;
		columnIndex: number;
		isLastColumn: boolean;
		personEmail: string;
		isWorkTooltipOpen: boolean;
		onWorkTooltipOpenChange: (isOpen: boolean) => void;
		onOpenDayDetail: (day: TeamStatusPersonDay) => void;
	};

	let {
		day,
		columnIndex,
		isLastColumn,
		personEmail,
		isWorkTooltipOpen,
		onWorkTooltipOpenChange,
		onOpenDayDetail
	}: Props = $props();

	function cellDividerClass(): string {
		if (columnIndex === 0) return isLastColumn ? 'border-r' : '';
		return isLastColumn ? 'border-l border-r' : 'border-l';
	}

	function cellToneClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.tone === 'working') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'finished') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'absent') return 'bg-background text-destructive';
		return 'text-muted-foreground';
	}

	function cellButtonClass(dayToStyle: TeamStatusPersonDay): string {
		return `relative flex h-full min-h-12 w-full max-w-none flex-col items-center justify-center gap-1 px-1.5 pb-2 text-center text-xs font-medium transition hover:bg-muted/30 focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${cellToneClass(dayToStyle)}`;
	}

	function segmentBarColor(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.locationColor ?? 'var(--color-muted-foreground)';
	}

	function absenceLabelClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.absenceTone === 'other') return 'text-muted-foreground';
		return 'text-info';
	}

	function absenceMeterClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.absenceTone === 'other') return 'bg-muted-foreground/30';
		return 'bg-info/30';
	}

	function daySegmentsTotalPercent(dayToMeasure: TeamStatusPersonDay): number {
		const totalPercent = dayToMeasure.segments.reduce(
			(total, segment) => total + segment.widthPercent,
			0
		);
		return Math.min(100, totalPercent);
	}

	const hasVisibleLabel = $derived(day.tone === 'absence' || day.label !== '-');
	const hasWorkTooltip = $derived(day.segments.length > 0);
	const buttonProps = $derived({
		class: cellButtonClass(day),
		'aria-label': hasVisibleLabel ? undefined : day.label,
		'data-testid': `team-status-cell-${personEmail}-${day.date}`,
		onclick: () => onOpenDayDetail(day),
	});
</script>

{#snippet CellButton({ props }: { props?: Record<string, unknown> })}
	{@const mergedProps = mergeProps(buttonProps, props ?? {})}
	<button type="button" {...mergedProps}>
		{#if day.tone === 'absence'}
			<span class={`min-w-0 max-w-full whitespace-normal break-all leading-tight ${absenceLabelClass(day)}`}>{day.label}</span>
		{:else if day.durationMinutes !== undefined}
			<DurationText minutes={day.durationMinutes} size="extraSmall" tone="default" />
		{:else if hasVisibleLabel}
			<span class="min-w-0 max-w-full whitespace-normal break-all leading-tight text-foreground">{day.label}</span>
		{/if}
		{#if day.detailLabel && day.segments.length === 0}
			<span class="max-w-full whitespace-normal break-all text-[10px] leading-tight text-foreground/70">
				{day.detailLabel}
			</span>
		{/if}
		{#if day.tone === 'absence'}
			<span class={`absolute inset-x-1.5 bottom-1 h-1.5 rounded-full ${absenceMeterClass(day)}`} aria-hidden="true"></span>
		{:else}
			<span class="absolute inset-x-1.5 bottom-1 h-1.5 rounded-full bg-muted" aria-hidden="true">
				{#if day.segments.length > 0}
					<span class="flex h-full overflow-hidden rounded-full" style:width={`${daySegmentsTotalPercent(day)}%`}>
						{#each day.segments as segment (segment.id)}
							<span
								class="h-full"
								style:flex-grow={segment.widthPercent}
								style:background-color={segmentBarColor(segment)}
							></span>
						{/each}
					</span>
				{/if}
			</span>
		{/if}
	</button>
{/snippet}

	{#snippet WorkSegmentTooltip()}
	<div class="grid min-w-44 gap-2">
		{#each day.segments as segment (segment.id)}
			<WorkSegmentSummary
				locationName={segment.locationName}
				locationColor={segment.locationColor}
				startTime={segment.startTime}
				endTime={segment.endTime}
				durationMinutes={segment.durationMinutes}
				isOpen={segment.isOpen}
			/>
		{/each}
	</div>
{/snippet}

<div class={`flex items-stretch justify-stretch text-center ${cellDividerClass()}`} role="cell">
	{#if hasWorkTooltip}
		<Tooltip.Root bind:open={() => isWorkTooltipOpen, onWorkTooltipOpenChange}>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					{@render CellButton({ props })}
				{/snippet}
			</Tooltip.Trigger>
			<Tooltip.Content
				side="top"
				sideOffset={6}
				class="mx-2 grid max-w-64 gap-2 bg-popover px-3 py-2 text-popover-foreground shadow-md ring-1 ring-border duration-150 ease-out data-[side=top]:slide-in-from-bottom-1"
				arrowClasses="bg-popover fill-popover"
			>
				{@render WorkSegmentTooltip()}
			</Tooltip.Content>
		</Tooltip.Root>
	{:else}
		{@render CellButton({})}
	{/if}
</div>
