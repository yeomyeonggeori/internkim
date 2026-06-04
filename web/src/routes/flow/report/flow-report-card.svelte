<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import type { FlowReportItem, FlowReportSection, FlowReportTone } from './flow-report-data';

	type Props = {
		section: FlowReportSection;
	};

	let { section }: Props = $props();
	let activeStatusIndex = $state<number | null>(null);

	const donutPalette = ['#2563eb', '#0f766e', '#7c3aed', '#64748b', '#334155'];

	function toneBarClass(tone: FlowReportTone): string {
		switch (tone) {
			case 'success':
				return 'bg-emerald-500';
			case 'active':
				return 'bg-sky-500';
			case 'request':
				return 'bg-violet-500';
			case 'planned':
				return 'bg-amber-500';
			case 'paused':
				return 'bg-orange-500';
			case 'stopped':
				return 'bg-rose-500';
			case 'member':
				return 'bg-teal-500';
			case 'business':
				return 'bg-blue-600';
			case 'type':
				return 'bg-indigo-600';
		}
	}

	function toneTextClass(tone: FlowReportTone): string {
		switch (tone) {
			case 'success':
				return 'text-emerald-700';
			case 'active':
				return 'text-sky-700';
			case 'request':
				return 'text-violet-700';
			case 'planned':
				return 'text-amber-700';
			case 'paused':
				return 'text-orange-700';
			case 'stopped':
				return 'text-rose-700';
			case 'member':
				return 'text-teal-700';
			case 'business':
				return 'text-blue-700';
			case 'type':
				return 'text-indigo-700';
		}
	}

	function statusSegmentClass(item: FlowReportItem, index: number): string {
		const startClass = index === 0 ? 'rounded-l-full' : '';
		const endClass = index === section.items.length - 1 ? 'rounded-r-full' : '';
		return `${toneBarClass(item.tone)} relative h-full focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${startClass} ${endClass}`;
	}

	function statusCardClass(index: number): string {
		const activeClass = activeStatusIndex === index ? 'border-foreground/20 shadow-sm' : 'border-border';
		return `grid min-h-[4.75rem] grid-cols-[3px_1fr] overflow-hidden rounded-lg border bg-card text-left text-sm transition duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring ${activeClass}`;
	}

	function treemapTileClass(item: FlowReportItem): string {
		if (item.percent >= 40) return 'bg-slate-100 text-slate-950 ring-slate-300';
		return 'bg-slate-50 text-slate-900 ring-slate-200';
	}

	function treemapColumnSpan(item: FlowReportItem): number {
		if (item.percent >= 40) return 2;
		return 1;
	}

	function treemapRowSpan(item: FlowReportItem): number {
		if (item.percent >= 40) return 2;
		return 1;
	}

	function itemWidth(value: number): number {
		return Math.max(4, (value / section.maxValue) * 100);
	}

	function donutColor(index: number): string {
		return donutPalette[index % donutPalette.length];
	}

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}

	function workloadScrollHint(itemCount: number): string {
		return `${itemCount}명 전체 · 목록 안에서 스크롤`;
	}

	function donutGradient(items: FlowReportItem[]): string {
		let cursor = 0;
		const segments = items.map((item, index) => {
			const start = cursor;
			const end = cursor + item.percent;
			cursor = end;
			return `${donutColor(index)} ${start}% ${end}%`;
		});
		return `conic-gradient(${segments.join(', ')})`;
	}
</script>

<Card.Root class="min-h-[18rem] min-w-0 w-full">
	<Card.Header class={section.chartKind === 'workload' ? 'pb-0' : 'pb-3'}>
		<div class="grid grid-cols-[1fr_auto] items-start gap-4">
			<div class="min-w-0 space-y-1">
				<Card.Title class="text-base">{section.title}</Card.Title>
				{#if section.description}
					<Card.Description class="text-sm leading-snug">{section.description}</Card.Description>
				{/if}
			</div>
			<div class="shrink-0 text-right">
				<div class="text-2xl font-semibold leading-none tabular-nums">{formatValue(section.total, section.unit)}</div>
				{#if section.chartKind === 'workload'}
					<div class="mt-1 text-xs font-medium text-muted-foreground tabular-nums">팀 평균: {formatValue(section.averageValue, section.unit)}</div>
				{/if}
			</div>
		</div>
	</Card.Header>
	<Card.Content>
		{#if section.items.length === 0}
			<div class="grid min-h-36 place-items-center rounded-md border border-dashed bg-muted/20 px-4 text-sm text-muted-foreground">
				{section.emptyLabel}
			</div>
		{:else if section.chartKind === 'stacked'}
			<div class="space-y-4">
				<div class="relative pt-6">
					<div class="flex h-3 rounded-full bg-muted">
						{#each section.items as item, index}
							<button
								type="button"
								class={statusSegmentClass(item, index)}
								style={`width: ${Math.max(3, item.percent)}%`}
								aria-label={`${item.label} ${formatValue(item.value, section.unit)} ${item.percent}%`}
								onmouseenter={() => (activeStatusIndex = index)}
								onmouseleave={() => (activeStatusIndex = null)}
								onfocus={() => (activeStatusIndex = index)}
								onblur={() => (activeStatusIndex = null)}
							></button>
						{/each}
					</div>
				</div>
				<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
					{#each section.items as item, index}
						<button
							type="button"
							class={statusCardClass(index)}
							onmouseenter={() => (activeStatusIndex = index)}
							onmouseleave={() => (activeStatusIndex = null)}
							onfocus={() => (activeStatusIndex = index)}
							onblur={() => (activeStatusIndex = null)}
						>
							<div class={toneBarClass(item.tone)}></div>
							<div class="grid content-center gap-1 px-3 py-2.5">
								<div class="flex min-w-0 items-baseline justify-between gap-2">
									<div class="truncate font-semibold text-foreground">{item.label}</div>
									{#if item.description}
										<div class={`shrink-0 text-[11px] font-medium ${toneTextClass(item.tone)}`}>{item.description}</div>
									{/if}
								</div>
								<div class="text-xs text-muted-foreground tabular-nums">{formatValue(item.value, section.unit)} · {item.percent}%</div>
							</div>
						</button>
					{/each}
				</div>
			</div>
		{:else if section.chartKind === 'workload'}
			<div class="-mt-2 space-y-2.5">
				<div class="relative">
					<div class="max-h-64 space-y-3.5 overflow-y-auto pr-1">
						{#each section.items as item}
							<div class="space-y-1.5">
								<div class="flex items-baseline justify-between gap-3 text-sm">
									<span class="min-w-0 truncate font-semibold">{item.label}</span>
									<span class={item.value > section.averageValue ? 'shrink-0 text-teal-700 tabular-nums' : 'shrink-0 text-muted-foreground tabular-nums'}>
										{formatValue(item.value, section.unit)}
									</span>
								</div>
								<div class="relative h-2 rounded-full bg-muted">
									<div class="h-full rounded-full bg-teal-500" style={`width: ${itemWidth(item.value)}%`}></div>
								</div>
							</div>
						{/each}
					</div>
					{#if section.items.length > 6}
						<div class="pointer-events-none absolute inset-x-0 bottom-0 h-8 bg-gradient-to-t from-card to-transparent"></div>
					{/if}
				</div>
				{#if section.items.length > 6}
					<div class="text-xs text-muted-foreground">{workloadScrollHint(section.items.length)}</div>
				{/if}
			</div>
		{:else if section.chartKind === 'donut'}
			<div class="grid min-h-44 items-center gap-5 sm:grid-cols-[10rem_1fr]">
				<div class="relative mx-auto size-32 rounded-full sm:size-36" style={`background: ${donutGradient(section.items)}`}>
					<div class="absolute inset-6 grid place-items-center rounded-full bg-card text-center">
						<div>
							<div class="text-2xl font-semibold leading-none tabular-nums">{formatValue(section.total, section.unit)}</div>
						</div>
					</div>
				</div>
				<div class="divide-y">
					{#each section.items as item, index}
						<div class="grid grid-cols-[1fr_auto] items-center gap-3 py-2 text-sm">
							<div class="flex min-w-0 items-center gap-2">
								<span class="size-2.5 shrink-0 rounded-full" style={`background: ${donutColor(index)}`}></span>
								<span class="truncate font-semibold">{item.label}</span>
							</div>
							<span class="text-right text-muted-foreground tabular-nums">{formatValue(item.value, section.unit)} · {item.percent}%</span>
						</div>
					{/each}
				</div>
			</div>
		{:else}
			<div class="grid auto-rows-[5.25rem] grid-cols-2 gap-2 sm:grid-cols-4">
				{#each section.items as item}
					<div
						class={`grid content-between rounded-lg p-3 ring-1 ${treemapTileClass(item)}`}
						style={`grid-column: span ${treemapColumnSpan(item)}; grid-row: span ${treemapRowSpan(item)}`}
					>
						<div class="truncate text-sm font-semibold">{item.label}</div>
						<div>
							<div class="text-lg font-semibold leading-none tabular-nums">{formatValue(item.value, section.unit)}</div>
							<div class="mt-1 text-xs text-muted-foreground tabular-nums">{item.percent}%</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
