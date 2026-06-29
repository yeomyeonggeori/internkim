<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import type { TeamStatusPersonDay } from './team-status-table-model';

	type Props = {
		day: TeamStatusPersonDay;
		index: number;
		personEmail: string;
		onOpenDayDetail: (day: TeamStatusPersonDay) => void;
	};

	let { day, index, personEmail, onOpenDayDetail }: Props = $props();

	function cellToneClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.tone === 'working') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'finished') return 'bg-background text-foreground';
		if (dayToStyle.tone === 'absence') return absenceBackgroundClass(dayToStyle);
		if (dayToStyle.tone === 'absent') return 'bg-background text-destructive';
		return 'text-muted-foreground';
	}

	function absenceBackgroundClass(dayToStyle: TeamStatusPersonDay): string {
		if (dayToStyle.absenceTone === 'other') return 'bg-background text-foreground';
		return 'bg-[color-mix(in_oklab,var(--color-info)_8%,var(--color-background))] text-foreground';
	}

	function cellButtonClass(dayToStyle: TeamStatusPersonDay): string {
		return `flex h-full min-h-16 w-full max-w-none flex-col items-center justify-center gap-1 px-1.5 text-center text-xs font-medium transition hover:bg-muted/30 focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring ${cellToneClass(dayToStyle)}`;
	}

	function cellTitle(dayToTitle: TeamStatusPersonDay): string {
		if (dayToTitle.locationName && dayToTitle.locationName !== dayToTitle.label) {
			return `${dayToTitle.label} · ${dayToTitle.locationName}`;
		}
		return dayToTitle.label;
	}

	function segmentBarColor(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.locationColor ?? 'hsl(var(--muted-foreground))';
	}

	function tooltipTimeLabel(segment: TeamStatusPersonDay['segments'][number]): string {
		return segment.durationLabel ? segment.timeLabel : segment.tooltipLabel;
	}
</script>

<div class={`flex items-stretch justify-stretch text-center ${index === 0 ? '' : 'border-l'}`} role="cell">
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				<button
					{...props}
					type="button"
					class={cellButtonClass(day)}
					title={cellTitle(day)}
					data-testid={`team-status-cell-${personEmail}-${day.date}`}
					onclick={() => onOpenDayDetail(day)}
				>
					<span class="min-w-0 max-w-full whitespace-normal break-all leading-tight text-foreground">{day.label}</span>
					{#if day.segments.length > 0}
						<span class="flex h-1.5 w-[88%] min-w-0 overflow-hidden rounded-full bg-muted" aria-hidden="true">
							{#each day.segments as segment (segment.id)}
								<span
									class="h-full min-w-1"
									style:width={`${segment.sharePercent}%`}
									style:background-color={segmentBarColor(segment)}
								></span>
							{/each}
						</span>
					{:else if day.detailLabel}
						<span class="max-w-full whitespace-normal break-all text-[10px] leading-tight text-foreground/70">
							{day.detailLabel}
						</span>
					{/if}
				</button>
			{/snippet}
		</Tooltip.Trigger>
		{#if day.segments.length > 0}
			<Tooltip.Content
				side="top"
				sideOffset={6}
				class="grid w-max max-w-[calc(100vw-2rem)] grid-cols-[0.375rem_max-content_max-content_max-content] gap-x-2 gap-y-1.5 overflow-x-auto"
			>
				{#each day.segments as segment (segment.id)}
					<div class="contents text-left tabular-nums">
						<span class="size-1.5 shrink-0 rounded-full" style:background-color={segmentBarColor(segment)}></span>
						<span class="whitespace-nowrap text-left">{segment.locationName}</span>
						<span class="whitespace-nowrap text-left">{tooltipTimeLabel(segment)}</span>
						<span class="whitespace-nowrap text-left">{segment.durationLabel ?? ''}</span>
					</div>
				{/each}
			</Tooltip.Content>
		{/if}
	</Tooltip.Root>
</div>
