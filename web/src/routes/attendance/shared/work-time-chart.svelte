<script lang="ts" module>
	let instanceCounter = 0;
</script>
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { getAttendanceState, type ChartMode } from '../attendance-context.svelte';
	import { attendanceText } from '../text';
	import {
		baselineY,
		buildAreaPath,
		buildSeries,
		PADDING_TOP,
		PADDING_X,
		plotPoints,
		type DailyValue,
		VIEW_HEIGHT,
		VIEW_WIDTH
	} from './work-time-chart-model';

	instanceCounter += 1;
	const gradientId = `work-time-chart-fill-${instanceCounter}`;

	type Props = {
		title: string;
		dailyValues: DailyValue[];
		formatValue: (value: number) => string;
	};

	type ChartModeOption = {
		value: ChartMode;
		label: string;
	};

	let {
		title,
		dailyValues,
		formatValue,
	}: Props = $props();

	const attendance = getAttendanceState();
	const text = createPageText(attendanceText);
	const modes = $derived<ChartModeOption[]>([
		{ value: 'day', label: text.day },
		{ value: 'week', label: text.week },
		{ value: 'month', label: text.month },
	]);

	const series = $derived(
		buildSeries(attendance.summary?.month ?? '', dailyValues, attendance.chartMode, {
			weekLabelTemplate: text.weekLabelTemplate,
			total: text.total
		})
	);

	const maxValue = $derived(Math.max(1, ...series.map((seriesPoint) => seriesPoint.value)));

	const plotted = $derived(plotPoints(series, maxValue));

	const linePath = $derived(
		plotted
			.map(
				(plottedPoint, index) =>
					`${index === 0 ? 'M' : 'L'} ${plottedPoint.x.toFixed(1)} ${plottedPoint.y.toFixed(1)}`
			)
			.join(' ')
	);

	const areaPath = $derived(buildAreaPath(plotted));

	const labelEvery = $derived(attendance.chartMode === 'day' && plotted.length > 16 ? 2 : 1);

	let hoveredIndex = $state<number | null>(null);
	let svgElement = $state<SVGSVGElement | null>(null);

	const hoveredPoint = $derived(hoveredIndex !== null ? plotted[hoveredIndex] : null);

	function handleMouseMove(event: MouseEvent) {
		if (!svgElement || plotted.length === 0) return;
		const rectangle = svgElement.getBoundingClientRect();
		const ratio = (event.clientX - rectangle.left) / rectangle.width;
		const viewX = ratio * VIEW_WIDTH;
		let nearestIndex = 0;
		let nearestDistance = Infinity;
		for (let i = 0; i < plotted.length; i += 1) {
			const distance = Math.abs(plotted[i].x - viewX);
			if (distance < nearestDistance) {
				nearestDistance = distance;
				nearestIndex = i;
			}
		}
		hoveredIndex = nearestIndex;
	}

	function handleMouseLeave() {
		hoveredIndex = null;
	}
</script>

<Card.Root>
	<Card.Header class="flex flex-row items-center justify-between space-y-0">
		<Card.Title class="flex items-center gap-1.5 text-sm">
			<ClockIcon class="size-3.5 text-muted-foreground" />
			{title}
		</Card.Title>
		<div class="flex items-center gap-1 rounded-md border p-0.5">
			{#each modes as mode (mode.value)}
				<Button
					variant={attendance.chartMode === mode.value ? 'secondary' : 'ghost'}
					size="sm"
					class="h-7 px-3 text-xs"
					onclick={() => (attendance.chartMode = mode.value)}
				>
					{mode.label}
				</Button>
			{/each}
		</div>
	</Card.Header>
	<Card.Content>
		<div class="relative">
			<svg
				bind:this={svgElement}
				viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
				class="text-primary aspect-[3/1] max-h-72 w-full"
				preserveAspectRatio="none"
				role="img"
				aria-label={title}
				onmousemove={handleMouseMove}
				onmouseleave={handleMouseLeave}
			>
				<defs>
					<linearGradient id={gradientId} x1="0" x2="0" y1="0" y2="1">
						<stop offset="0%" stop-color="currentColor" stop-opacity="0.25" />
						<stop offset="100%" stop-color="currentColor" stop-opacity="0.02" />
					</linearGradient>
				</defs>

				<line
					x1={PADDING_X}
					x2={VIEW_WIDTH - PADDING_X}
					y1={baselineY}
					y2={baselineY}
					class="text-foreground"
					stroke="currentColor"
					stroke-opacity="0.15"
					stroke-width="1"
				/>

				{#if plotted.length === 1}
					<circle cx={plotted[0].x} cy={plotted[0].y} r="6" fill="currentColor" />
				{:else}
					<path d={areaPath} fill={`url(#${gradientId})`} />
					<path
						d={linePath}
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linejoin="round"
						stroke-linecap="round"
					/>
					{#each plotted as plottedPoint, index (plottedPoint.point.key)}
						<circle
							cx={plottedPoint.x}
							cy={plottedPoint.y}
							r={hoveredIndex === index ? 5 : 3}
							fill="currentColor"
						/>
					{/each}
				{/if}

				{#if hoveredPoint}
					<line
						x1={hoveredPoint.x}
						x2={hoveredPoint.x}
						y1={PADDING_TOP}
						y2={baselineY}
						class="text-foreground"
						stroke="currentColor"
						stroke-opacity="0.2"
						stroke-width="1"
						stroke-dasharray="3 3"
						pointer-events="none"
					/>
				{/if}

				<g class="text-foreground">
					{#each plotted as plottedPoint, index (plottedPoint.point.key)}
						{#if index % labelEvery === 0}
							<text
								x={plottedPoint.x}
								y={VIEW_HEIGHT - 8}
								text-anchor="middle"
								font-size="11"
								fill="currentColor"
								opacity="0.55"
							>{plottedPoint.point.label}</text>
						{/if}
					{/each}
				</g>
			</svg>

			{#if hoveredPoint}
				<div
					class="border-border bg-popover text-popover-foreground pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-md border px-2 py-1 text-xs shadow-md"
					style={`left: ${(hoveredPoint.x / VIEW_WIDTH) * 100}%; top: ${(hoveredPoint.y / VIEW_HEIGHT) * 100}%; margin-top: -8px;`}
				>
					<div class="text-muted-foreground text-[10px] font-medium">{hoveredPoint.point.label}</div>
					<div class="font-semibold tabular-nums">{formatValue(hoveredPoint.point.value)}</div>
				</div>
			{/if}
		</div>
	</Card.Content>
</Card.Root>
