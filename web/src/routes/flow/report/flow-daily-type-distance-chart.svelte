<script lang="ts">
	import { flowDefinitionPaletteColor, flowTaskTypeColor } from '../flow-definition-colors';
	import type { FlowDefinitions } from '../flow-types';
	import type { FlowDailyTypeDistanceSection, FlowReportRow } from './flow-report-data';

	type Props = {
		section: FlowDailyTypeDistanceSection;
		definitions?: FlowDefinitions;
	};

	let { section, definitions }: Props = $props();
	let activeDailyIndex = $state<number | null>(null);

	const dailyGridTicks = [0, 12.5, 25, 37.5, 50, 62.5, 75, 87.5, 100];
	const dailyMajorTicks = [0, 25, 50, 75, 100];

	function typeSegmentColor(index: number, label = ''): string {
		if (!definitions || !label) return flowDefinitionPaletteColor(index);
		return flowTaskTypeColor(label, definitions);
	}

	function activeDailyRow(): FlowReportRow | null {
		if (activeDailyIndex === null) return null;
		return section.rows[activeDailyIndex] ?? null;
	}

	function dailyTooltipClass(index: number): string {
		const edgeClass = index >= section.rows.length - 2 ? '-translate-x-full -ml-3' : 'ml-3';
		return `pointer-events-none absolute top-3 z-20 min-w-40 rounded-md border bg-card/95 px-3 py-2 text-xs shadow-sm backdrop-blur ${edgeClass}`;
	}

	function dailyTooltipStyle(index: number): string {
		const columnCount = Math.max(1, section.rows.length);
		const columnCenter = ((index + 0.5) / columnCount) * 100;
		return `left: ${columnCenter}%`;
	}

	function dailyTickClass(value: number): string {
		return dailyMajorTicks.includes(value) ? 'bg-border/70' : 'bg-border/35';
	}

	function dailyTickLabel(value: number): string {
		return `${value}%`;
	}

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}
</script>

<div class="grid h-full min-h-0 gap-3 xl:grid-cols-[1fr_8.5rem]">
	<div class="min-w-0">
		<div class="grid grid-cols-[2.5rem_1fr] gap-2">
			<div class="relative h-72 text-[11px] text-muted-foreground">
				<div class="absolute inset-y-3 left-0 right-0">
					{#each dailyMajorTicks as tick}
						<div class="absolute right-0 tabular-nums" style={`bottom: ${tick}%; transform: translateY(50%)`}>{dailyTickLabel(tick)}</div>
					{/each}
				</div>
			</div>
			<div class="relative h-72 rounded-md border bg-card">
				<div class="absolute inset-x-3 inset-y-3">
					{#each dailyGridTicks as tick}
						<div class={`absolute inset-x-0 h-px ${dailyTickClass(tick)}`} style={`bottom: ${tick}%`}></div>
					{/each}
					<div class="relative z-10 grid h-full grid-cols-7 items-end gap-2">
						{#each section.rows as row, index}
							<button
								type="button"
								class="group flex h-full min-w-0 items-end justify-center rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
								aria-label={`${row.label} ${formatValue(row.total, section.unit)}`}
								onmouseenter={() => (activeDailyIndex = index)}
								onmouseleave={() => (activeDailyIndex = null)}
								onfocus={() => (activeDailyIndex = index)}
								onblur={() => (activeDailyIndex = null)}
							>
								<div class={`flex h-full w-full max-w-8 items-end rounded-sm ${row.total > 0 ? 'bg-muted/60' : 'bg-transparent'}`}>
									{#if row.total > 0}
										<div class="flex h-full w-full flex-col-reverse overflow-hidden rounded-sm">
											{#each row.segments as segment}
												<div
													style={`height: ${segment.percent}%; background: ${typeSegmentColor(segment.colorIndex, segment.label)}`}
													aria-label={`${row.label} ${segment.label} ${formatValue(segment.value, section.unit)}`}
												></div>
											{/each}
										</div>
									{:else}
										<div class="h-1 w-full rounded-full bg-muted"></div>
									{/if}
								</div>
							</button>
						{/each}
					</div>
				</div>
				{#if section.total === 0}
					<div class="absolute inset-0 grid place-items-center text-sm text-muted-foreground">{section.emptyLabel}</div>
				{/if}
				{#if activeDailyIndex !== null}
					{@const activeRow = activeDailyRow()}
					{#if activeRow && activeRow.total > 0}
						<div class={dailyTooltipClass(activeDailyIndex)} style={dailyTooltipStyle(activeDailyIndex)}>
							<div class="mb-1 flex items-baseline justify-between gap-4 font-medium text-foreground">
								<span>{activeRow.label}</span>
								<span class="tabular-nums">{formatValue(activeRow.total, section.unit)}</span>
							</div>
							<div class="grid gap-1 text-muted-foreground">
								{#each activeRow.segments as segment}
									<div class="flex items-center justify-between gap-4">
										<span class="flex min-w-0 items-center gap-1.5">
											<span class="size-2 shrink-0 rounded-sm" style={`background: ${typeSegmentColor(segment.colorIndex, segment.label)}`}></span>
											<span class="truncate">{segment.label}</span>
										</span>
										<span class="shrink-0 tabular-nums">{formatValue(segment.value, section.unit)}</span>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				{/if}
			</div>
			<div></div>
			<div class="grid grid-cols-7 gap-2 px-3 text-center text-xs text-muted-foreground">
				{#each section.rows as row}
					<div>{row.label}</div>
				{/each}
			</div>
		</div>
	</div>
	<div class="grid content-start gap-1.5 text-xs text-muted-foreground">
		{#each section.items as item, index}
			<div class="flex min-w-0 items-center justify-between gap-2">
				<div class="flex min-w-0 items-center gap-2">
					<span class="size-2.5 shrink-0 rounded-sm" style={`background: ${typeSegmentColor(item.colorIndex ?? index, item.label)}`}></span>
					<span class="truncate">{item.label}</span>
				</div>
				<span class="shrink-0 tabular-nums">{formatValue(item.value, section.unit)}</span>
			</div>
		{/each}
	</div>
</div>
