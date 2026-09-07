<script lang="ts">
	import ColorMarker from '$lib/components/color-marker.svelte';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import type { CRMKPICardData, CRMKPIMoneyDetail, CRMKPISegment } from './crm-kpi';
	import { segmentFillsOf } from './crm-kpi-tone';

	type Props = {
		card: CRMKPICardData;
	};

	type MoneyValueProps = {
		displayValue: string;
		label: string;
		moneyDetails?: CRMKPIMoneyDetail[];
		className: string;
	};

	let { card }: Props = $props();

	const segmentFills = $derived(segmentFillsOf(card.segments));

	function segmentValueClass(segment: CRMKPISegment): string {
		const base = 'shrink-0 font-medium tabular-nums';
		if (segment.tone === 'attention' && segment.value > 0) return `${base} text-destructive`;
		return base;
	}
</script>

{#snippet MoneyValue({ displayValue, label, moneyDetails, className }: MoneyValueProps)}
	{#if moneyDetails && moneyDetails.length > 1}
		<HoverCard.Root openDelay={150}>
			<HoverCard.Trigger>
				{#snippet child({ props })}
					<button
						{...props}
						type="button"
						class={`${className} rounded-sm border-0 bg-transparent p-0 font-[inherit] text-inherit focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring`}
						aria-label={`${label}: ${displayValue}`}
						data-crm-money-summary={label}
					>
						{displayValue}
					</button>
				{/snippet}
			</HoverCard.Trigger>
			<HoverCard.Content
				side="top"
				align="center"
				sideOffset={8}
				class="w-48 p-3"
				data-crm-money-popover={label}
			>
				<p class="mb-2 text-xs font-medium">{label}</p>
				<div class="grid gap-1.5">
					{#each moneyDetails as detail (detail.currency)}
						<div class="flex items-center justify-between gap-4 text-xs">
							<span class="text-muted-foreground">{detail.currency}</span>
							<span class="font-medium tabular-nums">{detail.displayValue}</span>
						</div>
					{/each}
				</div>
			</HoverCard.Content>
		</HoverCard.Root>
	{:else}
		<span class={className}>{displayValue}</span>
	{/if}
{/snippet}

<div data-crm-kpi={card.id} class="flex min-w-0 flex-col gap-3 bg-card p-4">
	<h2 class="text-xs font-medium text-muted-foreground">{card.totalLabel}</h2>

	<p data-crm-kpi-value class="min-w-0 text-2xl font-semibold tracking-tight tabular-nums">
		{@render MoneyValue({
			displayValue: card.totalValue,
			label: card.totalLabel,
			moneyDetails: card.totalMoneyDetails,
			className: 'block min-w-0'
		})}
	</p>

	<div data-crm-kpi-chart class="flex h-1.5 w-full overflow-hidden rounded-full bg-muted">
		{#each segmentFills as fill (fill.segment.label)}
			<span class={fill.fillClass} style={`flex-basis: ${fill.percent}%; ${fill.fillStyle}`}></span>
		{/each}
	</div>

	<div class="grid min-w-0 gap-1.5">
		{#each segmentFills as fill (fill.segment.label)}
			<div class="flex min-w-0 items-center gap-2 text-xs">
				<ColorMarker class={fill.fillClass} color={fill.segment.color} />
				<span class="min-w-0 flex-1 truncate text-muted-foreground">{fill.segment.label}</span>
				{@render MoneyValue({
					displayValue: fill.segment.displayValue,
					label: fill.segment.label,
					moneyDetails: fill.segment.moneyDetails,
					className: segmentValueClass(fill.segment)
				})}
			</div>
		{/each}
	</div>
</div>
