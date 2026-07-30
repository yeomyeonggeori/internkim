<script lang="ts">
	import { flowBusinessColor, flowDefinitionPaletteColor } from '../flow-definition-colors';
	import type { FlowDefinitions } from '../flow-types';
	import type { FlowBusinessDistanceSection, FlowReportItem } from './flow-report-data';

	type Props = {
		section: FlowBusinessDistanceSection;
		definitions?: FlowDefinitions;
	};

	let { section, definitions }: Props = $props();

	function donutColor(index: number, label: string): string {
		if (!definitions) return flowDefinitionPaletteColor(index);
		return flowBusinessColor(label, definitions);
	}

	function formatValue(value: number, unit: string): string {
		return `${value}${unit}`;
	}

	function donutGradient(items: FlowReportItem[]): string {
		let cursor = 0;
		const segments = items.map((item, index) => {
			const start = cursor;
			const end = cursor + item.percent;
			cursor = end;
			return `${donutColor(index, item.label)} ${start}% ${end}%`;
		});
		return `conic-gradient(${segments.join(', ')})`;
	}
</script>

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
					<span class="size-2.5 shrink-0 rounded-full" style={`background: ${donutColor(index, item.label)}`}></span>
					<span class="truncate font-semibold">{item.label}</span>
				</div>
				<span class="text-right text-muted-foreground tabular-nums">{formatValue(item.value, section.unit)} · {item.percent}%</span>
			</div>
		{/each}
	</div>
</div>
