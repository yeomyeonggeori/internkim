<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import SegmentTooltip from '../shared/segment-tooltip.svelte';
	import type { TeamStatusPersonDay } from './team-status-table-model';

	type Props = {
		day: TeamStatusPersonDay;
		index: number;
		personEmail: string;
		canShowTooltip: boolean;
		onOpenDayDetail: (day: TeamStatusPersonDay) => void;
	};

	let { day, index, personEmail, canShowTooltip, onOpenDayDetail }: Props = $props();

	function cellToneClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.tone === 'working') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'finished') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'absent') return 'bg-background text-destructive';
		return 'text-muted-foreground';
	}

	function cellButtonClass(dayToStyle: TeamStatusPersonDay): string {
		return `relative flex h-full min-h-12 w-full max-w-none flex-col items-center justify-center gap-1 px-1.5 pb-2 text-center text-xs font-medium transition hover:bg-muted/30 focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring sm:min-h-16 ${cellToneClass(dayToStyle)}`;
	}

	function cellTitle(dayToTitle: TeamStatusPersonDay): string {
		if (dayToTitle.locationName && dayToTitle.locationName !== dayToTitle.label) {
			return `${dayToTitle.label} · ${dayToTitle.locationName}`;
		}
		return dayToTitle.label;
	}

	function segmentBarColor(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.locationColor ?? 'var(--color-muted-foreground)';
	}

	function tooltipTimeLabel(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.durationLabel ? segment.timeLabel : segment.tooltipLabel;
	}

	function absenceBadgeClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.absenceTone === 'other') return '';
		return 'border-info/40 bg-info/10 text-info';
	}

	function daySegmentsTotalPercent(dayToMeasure: TeamStatusPersonDay): number {
		const totalPercent = dayToMeasure.segments.reduce(
			(total, segment) => total + segment.widthPercent,
			0
		);
		return Math.min(100, totalPercent);
	}

	const tooltipRows = $derived(
		day.segments.map((segment) => ({
			id: segment.id,
			color: segment.locationColor,
			locationName: segment.locationName,
			timeLabel: tooltipTimeLabel(segment),
			durationLabel: segment.durationLabel,
		}))
	);
</script>

<div class={`flex items-stretch justify-stretch text-center ${index === 0 ? '' : 'border-l'}`} role="cell">
	{#if canShowTooltip}
		<Tooltip.Root>
			<Tooltip.Trigger>
				{#snippet child({ props })}
					{@render statusButton(props, cellTitle(day))}
				{/snippet}
			</Tooltip.Trigger>
			{#if day.segments.length > 0}
				<Tooltip.Content
					side="top"
					sideOffset={6}
					class="w-max max-w-[calc(100vw-2rem)] border bg-popover text-popover-foreground shadow-md"
					arrowClasses="hidden"
				>
					<SegmentTooltip rows={tooltipRows} />
				</Tooltip.Content>
			{/if}
		</Tooltip.Root>
	{:else}
		{@render statusButton({}, undefined)}
	{/if}
</div>

{#snippet statusButton(triggerProps: Record<string, unknown>, titleText: string | undefined)}
	<button
		{...triggerProps}
		type="button"
		class={cellButtonClass(day)}
		title={titleText}
		data-testid={`team-status-cell-${personEmail}-${day.date}`}
		onclick={() => onOpenDayDetail(day)}
	>
		{#if day.tone === 'absence'}
			<Badge variant="outline" class={absenceBadgeClass(day)}>{day.label}</Badge>
		{:else}
			<span class="min-w-0 max-w-full whitespace-normal break-all leading-tight text-foreground">{day.label}</span>
		{/if}
		{#if day.segments.length > 0}
			<span class="absolute inset-x-1.5 bottom-1 h-[3px] rounded-full bg-muted" aria-hidden="true">
				<span class="flex h-full overflow-hidden rounded-full" style:width={`${daySegmentsTotalPercent(day)}%`}>
					{#each day.segments as segment (segment.id)}
						<span
							class="h-full"
							style:flex-grow={segment.widthPercent}
							style:background-color={segmentBarColor(segment)}
						></span>
					{/each}
				</span>
			</span>
		{:else if day.detailLabel}
			<span class="max-w-full whitespace-normal break-all text-[10px] leading-tight text-foreground/70">
				{day.detailLabel}
			</span>
		{/if}
	</button>
{/snippet}
