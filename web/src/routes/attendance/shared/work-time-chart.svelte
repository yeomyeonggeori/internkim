<script lang="ts" module>
	let instanceCounter = 0;
</script>
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import { getAttendanceState, type ChartMode } from '../attendance-context.svelte';
	import { eachDayOfMonth, isoWeekStart } from './attendance-date';

	instanceCounter += 1;
	const gradientId = `work-time-chart-fill-${instanceCounter}`;

	type DailyValue = { date: string; value: number };

	let {
		title,
		dailyValues,
		formatValue,
	}: {
		title: string;
		dailyValues: DailyValue[];
		formatValue: (value: number) => string;
	} = $props();

	const attendance = getAttendanceState();
	const modes: { value: ChartMode; label: string }[] = [
		{ value: 'day', label: '일별' },
		{ value: 'week', label: '주별' },
		{ value: 'month', label: '월별' },
	];

	type Point = { key: string; label: string; value: number };

	const series = $derived(buildSeries(attendance.summary?.month ?? '', dailyValues, attendance.chartMode));

	function buildSeries(month: string, daily: DailyValue[], mode: ChartMode): Point[] {
		if (!month) return [];
		const valueByDate = new Map(daily.map((d) => [d.date, d.value]));
		const allDays = eachDayOfMonth(month);
		const dailyPoints = allDays.map((date) => ({ date, value: valueByDate.get(date) ?? 0 }));

		if (mode === 'day') {
			return dailyPoints.map((d) => ({
				key: d.date,
				label: d.date.slice(8, 10),
				value: d.value,
			}));
		}
		if (mode === 'week') {
			const buckets = new Map<string, number>();
			for (const d of dailyPoints) {
				const key = isoWeekStart(d.date);
				buckets.set(key, (buckets.get(key) ?? 0) + d.value);
			}
			return [...buckets.entries()]
				.sort((a, b) => a[0].localeCompare(b[0]))
				.map(([key, value], idx) => ({ key, label: `${idx + 1}주`, value }));
		}
		const total = dailyPoints.reduce((sum, d) => sum + d.value, 0);
		return [{ key: 'total', label: '합계', value: total }];
	}

	const VIEW_W = 1000;
	const VIEW_H = 220;
	const PAD_X = 24;
	const PAD_TOP = 16;
	const PAD_BOTTOM = 28;

	const innerW = VIEW_W - PAD_X * 2;
	const innerH = VIEW_H - PAD_TOP - PAD_BOTTOM;
	const baselineY = VIEW_H - PAD_BOTTOM;

	const maxValue = $derived(Math.max(1, ...series.map((p) => p.value)));

	type Plotted = { x: number; y: number; point: Point };

	const plotted = $derived(plotPoints(series, maxValue));

	function plotPoints(points: Point[], max: number): Plotted[] {
		const n = points.length;
		return points.map((point, idx) => {
			const x = n === 1
				? PAD_X + innerW / 2
				: PAD_X + (innerW * idx) / (n - 1);
			const ratio = point.value / max;
			const y = baselineY - ratio * innerH;
			return { x, y, point };
		});
	}

	const linePath = $derived(
		plotted.map((p, idx) => `${idx === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')
	);

	const areaPath = $derived(buildAreaPath(plotted));

	function buildAreaPath(points: Plotted[]): string {
		if (!points.length) return '';
		const head = `M ${points[0].x.toFixed(1)} ${baselineY}`;
		const top = points.map((p) => `L ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
		const tail = `L ${points[points.length - 1].x.toFixed(1)} ${baselineY} Z`;
		return `${head} ${top} ${tail}`;
	}

	const labelEvery = $derived(attendance.chartMode === 'day' && plotted.length > 16 ? 2 : 1);

	let hoveredIndex = $state<number | null>(null);
	let svgEl = $state<SVGSVGElement | null>(null);

	const hoveredPoint = $derived(hoveredIndex !== null ? plotted[hoveredIndex] : null);

	function handleMouseMove(event: MouseEvent) {
		if (!svgEl || plotted.length === 0) return;
		const rect = svgEl.getBoundingClientRect();
		const ratio = (event.clientX - rect.left) / rect.width;
		const viewX = ratio * VIEW_W;
		let nearestIdx = 0;
		let nearestDist = Infinity;
		for (let i = 0; i < plotted.length; i += 1) {
			const dist = Math.abs(plotted[i].x - viewX);
			if (dist < nearestDist) {
				nearestDist = dist;
				nearestIdx = i;
			}
		}
		hoveredIndex = nearestIdx;
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
				bind:this={svgEl}
				viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
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
					x1={PAD_X}
					x2={VIEW_W - PAD_X}
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
					{#each plotted as p, idx (p.point.key)}
						<circle
							cx={p.x}
							cy={p.y}
							r={hoveredIndex === idx ? 5 : 3}
							fill="currentColor"
						/>
					{/each}
				{/if}

				{#if hoveredPoint}
					<line
						x1={hoveredPoint.x}
						x2={hoveredPoint.x}
						y1={PAD_TOP}
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
					{#each plotted as p, idx (p.point.key)}
						{#if idx % labelEvery === 0}
							<text
								x={p.x}
								y={VIEW_H - 8}
								text-anchor="middle"
								font-size="11"
								fill="currentColor"
								opacity="0.55"
							>{p.point.label}</text>
						{/if}
					{/each}
				</g>
			</svg>

			{#if hoveredPoint}
				<div
					class="border-border bg-popover text-popover-foreground pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-md border px-2 py-1 text-xs shadow-md"
					style={`left: ${(hoveredPoint.x / VIEW_W) * 100}%; top: ${(hoveredPoint.y / VIEW_H) * 100}%; margin-top: -8px;`}
				>
					<div class="text-muted-foreground text-[10px] font-medium">{hoveredPoint.point.label}</div>
					<div class="font-semibold tabular-nums">{formatValue(hoveredPoint.point.value)}</div>
				</div>
			{/if}
		</div>
	</Card.Content>
</Card.Root>
