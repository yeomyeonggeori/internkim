<script lang="ts">
	import { formatCompactMoney } from '$lib/money';
	import { dataRoomCompanyText } from './company-text';
	import { humanizedKey, recordAttributes, type CompanyRecord } from './company';

	let { round, locale }: { round: CompanyRecord; locale: 'ko' | 'en' } = $props();

	const text = $derived(dataRoomCompanyText[locale]);
	const attributes = $derived(recordAttributes(round));
	const committedShare = $derived(
		round.amount && round.committed && round.committed.currency === round.amount.currency && round.amount.amount > 0
			? Math.min(1, round.committed.amount / round.amount.amount)
			: undefined
	);
	const percent = $derived(committedShare === undefined ? '' : new Intl.NumberFormat(locale, { style: 'percent' }).format(committedShare));
</script>

<section class="bg-card grid gap-4 rounded-lg border p-5" aria-labelledby="company-raise">
	<div class="flex flex-wrap items-start justify-between gap-x-6 gap-y-2">
		<div class="grid gap-1">
			<p class="text-muted-foreground inline-flex items-center gap-1.5 text-xs">
				<span class="bg-success size-1.5 rounded-full" aria-hidden="true"></span>{text.raise.label}
			</p>
			<h3 id="company-raise" class="text-lg font-semibold">{round.title}</h3>
		</div>
		{#if round.amount}
			<div class="grid gap-1 text-right">
				<p class="text-muted-foreground text-xs">{text.raise.target}</p>
				<p class="text-2xl font-semibold tracking-tight tabular-nums">{formatCompactMoney(round.amount, locale)}</p>
			</div>
		{/if}
	</div>
	{#if committedShare !== undefined && round.committed}
		<div class="grid gap-1.5">
			<div class="bg-muted h-2 overflow-hidden rounded-full" role="img" aria-label={text.raise.committed(formatCompactMoney(round.committed, locale), percent)}>
				<div class="bg-foreground h-full rounded-full" style:width={`${committedShare * 100}%`}></div>
			</div>
			<p class="text-sm tabular-nums"><span class="font-medium">{text.raise.committed(formatCompactMoney(round.committed, locale), percent)}</span></p>
		</div>
	{/if}
	{#if round.detail || round.date || attributes.length > 0}
		<dl class="text-muted-foreground flex flex-wrap gap-x-5 gap-y-1 text-sm">
			{#if round.date}<div class="flex gap-1.5"><dt>{text.raise.closes}</dt><dd class="text-foreground tabular-nums">{round.date.slice(0, 7).replace('-', '.')}</dd></div>{/if}
			{#each attributes as [key, value] (key)}
				<div class="flex gap-1.5"><dt>{text.attributeNames[key] ?? humanizedKey(key)}</dt><dd class="text-foreground">{value}</dd></div>
			{/each}
		</dl>
		{#if round.detail}<p class="text-muted-foreground text-sm leading-relaxed text-pretty">{round.detail}</p>{/if}
	{/if}
</section>
