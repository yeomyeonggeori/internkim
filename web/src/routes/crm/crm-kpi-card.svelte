<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as HoverCard from '$lib/components/ui/hover-card';
	import type { CRMKPICardData, CRMKPIMoneyDetail } from './crm-kpi';

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

	function donutBackground(): string {
		const total = card.segments.reduce((sum, segment) => sum + segment.value, 0);
		if (total <= 0) return 'conic-gradient(var(--muted) 0% 100%)';

		let cursor = 0;
		const segments = card.segments.map((segment) => {
			const start = cursor;
			const end = cursor + (segment.value / total) * 100;
			cursor = end;
			return `${segment.color} ${start}% ${end}%`;
		});
		return `conic-gradient(${segments.join(', ')})`;
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

<Card.Root data-crm-kpi={card.id} class="h-48 min-w-0 gap-0 overflow-hidden py-0 ring-inset">
	<div class="flex h-full flex-col p-4 text-left">
		<div class="min-w-0">
			<h2 class="text-sm font-semibold">{card.title}</h2>
			<p class="mt-1 line-clamp-2 min-h-8 text-xs leading-4 text-muted-foreground">{card.description}</p>
		</div>

		<div class="mt-3 grid min-h-0 flex-1 grid-cols-[6rem_minmax(0,1fr)] items-center gap-4">
			<div class="grid justify-items-center">
				<div data-crm-kpi-chart class="relative size-24 rounded-full" style={`background: ${donutBackground()}`}>
					<div class="absolute inset-[0.45rem] flex flex-col items-center justify-center gap-0.5 rounded-full bg-card px-1 text-center">
						<p data-crm-kpi-value class="max-w-full text-xl font-semibold leading-6 tabular-nums">
							{@render MoneyValue({
								displayValue: card.totalValue,
								label: card.totalLabel,
								moneyDetails: card.totalMoneyDetails,
								className: 'block max-w-full truncate'
							})}
						</p>
						<p data-crm-kpi-label class="max-w-full text-[0.625rem] leading-3 text-muted-foreground">{card.totalLabel}</p>
					</div>
				</div>
			</div>

			<div class="grid min-w-0 gap-2">
				{#each card.segments as segment (segment.label)}
					<div class="flex min-w-0 items-center gap-2 text-xs">
						<span class="size-2 shrink-0 rounded-full" style={`background: ${segment.color}`}></span>
						<span class="min-w-0 flex-1 truncate text-muted-foreground">{segment.label}</span>
						{@render MoneyValue({
							displayValue: segment.displayValue,
							label: segment.label,
							moneyDetails: segment.moneyDetails,
							className: 'shrink-0 font-medium tabular-nums'
						})}
					</div>
				{/each}
			</div>
		</div>
	</div>
</Card.Root>
