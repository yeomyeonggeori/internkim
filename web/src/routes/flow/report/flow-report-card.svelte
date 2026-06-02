<!-- Flow 보고 탭의 카드별 통계 시각화를 렌더링합니다. -->
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import type { FlowReportItem, FlowReportSection, FlowReportTone } from './flow-report-data';

	type Props = {
		section: FlowReportSection;
	};

	let { section }: Props = $props();

	const palette = ['#2563eb', '#0f766e', '#7c3aed', '#f59e0b', '#e11d48', '#475569'];

	function toneClass(tone: FlowReportTone): string {
		switch (tone) {
			case 'success':
				return 'bg-emerald-500';
			case 'active':
				return 'bg-sky-500';
			case 'request':
				return 'bg-violet-500';
			case 'planned':
				return 'bg-amber-500';
			case 'blocked':
				return 'bg-rose-500';
			case 'member':
				return 'bg-teal-500';
			case 'business':
				return 'bg-blue-500';
			case 'type':
				return 'bg-indigo-500';
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
			case 'blocked':
				return 'text-rose-700';
			case 'member':
				return 'text-teal-700';
			case 'business':
				return 'text-blue-700';
			case 'type':
				return 'text-indigo-700';
		}
	}

	function toneBackgroundClass(tone: FlowReportTone): string {
		switch (tone) {
			case 'success':
				return 'bg-emerald-50 text-emerald-800 ring-emerald-200';
			case 'active':
				return 'bg-sky-50 text-sky-800 ring-sky-200';
			case 'request':
				return 'bg-violet-50 text-violet-800 ring-violet-200';
			case 'planned':
				return 'bg-amber-50 text-amber-800 ring-amber-200';
			case 'blocked':
				return 'bg-rose-50 text-rose-800 ring-rose-200';
			case 'member':
				return 'bg-teal-50 text-teal-800 ring-teal-200';
			case 'business':
				return 'bg-blue-50 text-blue-800 ring-blue-200';
			case 'type':
				return 'bg-indigo-50 text-indigo-800 ring-indigo-200';
		}
	}

	function itemColor(index: number): string {
		return palette[index % palette.length];
	}

	function treemapColumnSpan(item: FlowReportItem): number {
		if (item.percent >= 40) return 2;
		return 1;
	}

	function treemapRowSpan(item: FlowReportItem): number {
		if (item.percent >= 40) return 2;
		return 1;
	}

	function donutGradient(items: FlowReportItem[]): string {
		let cursor = 0;
		const segments = items.map((item, index) => {
			const start = cursor;
			const end = cursor + item.percent;
			cursor = end;
			return `${itemColor(index)} ${start}% ${end}%`;
		});
		return `conic-gradient(${segments.join(', ')})`;
	}
</script>

<Card.Root class="min-h-[17.5rem]">
	<Card.Header>
		<div class="grid grid-cols-[1fr_auto] items-start gap-3">
			<div class="min-w-0 space-y-1">
				<Card.Title>{section.title}</Card.Title>
				{#if section.description}
					<Card.Description>{section.description}</Card.Description>
				{/if}
			</div>
			<div class="shrink-0 text-right">
				<div class="text-xl font-semibold leading-none tabular-nums">{section.total}</div>
				<div class="mt-1 text-[11px] text-muted-foreground">{section.unit}</div>
			</div>
		</div>
	</Card.Header>
	<Card.Content>
		{#if section.items.length === 0}
			<div class="rounded-lg border border-dashed bg-muted/20 px-3 py-8 text-center text-sm text-muted-foreground">
				{section.emptyLabel}
			</div>
		{:else if section.chartKind === 'stacked'}
			<div class="space-y-3">
				<div class="flex h-4 overflow-hidden rounded-full bg-muted">
					{#each section.items as item}
						<div
							class={toneClass(item.tone)}
							style={`width: ${Math.max(3, item.percent)}%`}
							aria-label={`${item.label} ${item.value}${section.unit}`}
						></div>
					{/each}
				</div>
				<div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
					{#each section.items as item}
						<div class={`grid h-14 content-between rounded-lg px-2.5 py-2 text-xs ring-1 ${toneBackgroundClass(item.tone)}`}>
							<div class="truncate font-medium">{item.label}</div>
							<div class="tabular-nums">{item.value}{section.unit} · {item.percent}%</div>
						</div>
					{/each}
				</div>
				{#if section.alertValue > 0}
					<div class="rounded-lg bg-rose-50 px-3 py-2 text-xs font-medium text-rose-800 ring-1 ring-rose-200">
						멈춘 일 {section.alertValue}{section.unit}를 먼저 확인해야 합니다.
					</div>
				{/if}
			</div>
		{:else if section.chartKind === 'workload'}
			<div class="space-y-3">
				<div class="flex items-center justify-between rounded-lg bg-muted/35 px-3 py-2 text-xs text-muted-foreground">
					<span>팀 평균</span>
					<span class="font-medium tabular-nums text-foreground">{section.averageValue}{section.unit}</span>
				</div>
				<div class="space-y-2.5">
					{#each section.items as item}
						<div class="grid h-11 content-between gap-1.5">
							<div class="flex items-center justify-between gap-3 text-sm">
								<div class="min-w-0 truncate font-medium">{item.label}</div>
								<div class={`shrink-0 text-xs tabular-nums ${item.value > section.averageValue ? 'text-teal-700' : 'text-muted-foreground'}`}>
									{item.value}{section.unit}
								</div>
							</div>
							<div class="relative h-2 rounded-full bg-muted">
								<div
									class="absolute inset-y-[-3px] w-px bg-foreground/30"
									style={`left: ${Math.min(100, (section.averageValue / section.maxValue) * 100)}%`}
								></div>
								<div
									class="h-full rounded-full bg-teal-500"
									style={`width: ${Math.max(4, (item.value / section.maxValue) * 100)}%`}
								></div>
							</div>
						</div>
					{/each}
				</div>
			</div>
		{:else if section.chartKind === 'donut'}
			<div class="grid min-h-44 items-center gap-4 sm:grid-cols-[11rem_1fr]">
				<div class="relative mx-auto size-32 rounded-full sm:size-36" style={`background: ${donutGradient(section.items)}`}>
					<div class="absolute inset-6 grid place-items-center rounded-full bg-card text-center">
						<div class="text-2xl font-semibold tabular-nums">{section.total}</div>
						<div class="text-[11px] text-muted-foreground">{section.unit}</div>
					</div>
				</div>
				<div class="space-y-2">
					{#each section.items as item, index}
						<div class="grid grid-cols-[1fr_auto] items-center gap-3 text-sm">
							<div class="flex min-w-0 items-center gap-2">
								<span class="size-2.5 shrink-0 rounded-full" style={`background: ${itemColor(index)}`}></span>
								<span class="truncate font-medium">{item.label}</span>
							</div>
							<span class="text-right text-xs text-muted-foreground tabular-nums">{item.value}{section.unit} · {item.percent}%</span>
						</div>
					{/each}
				</div>
			</div>
		{:else}
			<div class="grid auto-rows-[5.5rem] grid-cols-2 gap-2 sm:grid-cols-4">
				{#each section.items as item}
					<div
						class={`grid content-between rounded-lg p-3 ring-1 ${toneBackgroundClass(item.tone)}`}
						style={`grid-column: span ${treemapColumnSpan(item)}; grid-row: span ${treemapRowSpan(item)}`}
					>
						<div class="truncate text-sm font-semibold">{item.label}</div>
						<div>
							<div class="text-2xl font-semibold leading-none tabular-nums">{item.value}</div>
							<div class={`mt-1 text-xs tabular-nums ${toneTextClass(item.tone)}`}>{item.percent}% · {section.unit}</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Card.Content>
</Card.Root>
