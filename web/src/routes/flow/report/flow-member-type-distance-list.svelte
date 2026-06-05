<script lang="ts">
	import { tick } from 'svelte';
	import { flowTypeColor } from '../flow-report-colors';
	import type { FlowReportSection } from './flow-report-data';

	type Props = {
		section: FlowReportSection;
	};

	let { section }: Props = $props();
	let rowScrollViewport = $state<HTMLElement | null>(null);
	let hasRowScrollOverflow = $state(false);
	let isRowScrollAtEnd = $state(true);

	const rowScrollFadeThreshold = 16;

	$effect(() => {
		section.rows.length;
		tick().then(updateRowScrollFade);
	});

	function typeSegmentColor(index: number): string {
		return flowTypeColor(index);
	}

	function rowWidth(value: number): number {
		return Math.max(4, (value / section.maxValue) * 100);
	}

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}

	function workloadScrollHint(itemCount: number): string {
		return `${itemCount}명 전체 · 목록 안에서 스크롤`;
	}

	function updateRowScrollFade(): void {
		if (!rowScrollViewport) {
			hasRowScrollOverflow = false;
			isRowScrollAtEnd = true;
			return;
		}

		const overflowAmount = rowScrollViewport.scrollHeight - rowScrollViewport.clientHeight;
		const scrollRemaining = overflowAmount - rowScrollViewport.scrollTop;
		hasRowScrollOverflow = overflowAmount > rowScrollFadeThreshold;
		isRowScrollAtEnd = scrollRemaining <= 2;
	}

	function shouldShowRowScrollFade(): boolean {
		return hasRowScrollOverflow && !isRowScrollAtEnd;
	}
</script>

<div class="-mt-1 flex h-full min-h-0 flex-col gap-1.5">
	<div class="relative min-h-0 flex-1">
		<div bind:this={rowScrollViewport} class="h-full min-h-28 space-y-2 overflow-y-auto pb-8 pr-1" onscroll={updateRowScrollFade}>
			{#each section.rows as row}
				<div class="space-y-1">
					<div class="flex items-baseline justify-between gap-3 text-sm">
						<span class="min-w-0 truncate font-semibold">{row.label}</span>
						<span class={row.total > section.averageValue ? 'shrink-0 text-teal-700 tabular-nums' : 'shrink-0 text-muted-foreground tabular-nums'}>
							{formatValue(row.total, section.unit)} · {row.percent}%
						</span>
					</div>
					<div class="h-2 overflow-hidden rounded-full bg-muted" style={`width: ${rowWidth(row.total)}%`}>
						<div class="flex h-full w-full">
							{#each row.segments as segment}
								<div
									style={`width: ${Math.max(4, segment.percent)}%; background: ${typeSegmentColor(segment.colorIndex)}`}
									aria-label={`${row.label} ${segment.label} ${formatValue(segment.value, section.unit)}`}
								></div>
							{/each}
						</div>
					</div>
					<div class="flex flex-wrap gap-x-2.5 gap-y-0.5 text-[11px] leading-none text-muted-foreground">
						{#each row.segments as segment}
							<div class="flex items-center gap-1.5">
								<span class="size-2 rounded-full" style={`background: ${typeSegmentColor(segment.colorIndex)}`}></span>
								<span>{segment.label} {formatValue(segment.value, section.unit)}</span>
							</div>
						{/each}
					</div>
				</div>
			{/each}
		</div>
		{#if shouldShowRowScrollFade()}
			<div class="pointer-events-none absolute inset-x-0 bottom-0 h-8 rounded-b-md bg-gradient-to-t from-card via-card/75 to-transparent"></div>
		{/if}
	</div>
	{#if section.rows.length > 4}
		<div class="text-xs text-muted-foreground">{workloadScrollHint(section.rows.length)}</div>
	{/if}
</div>
