<script lang="ts">
	import DollarSignIcon from '@lucide/svelte/icons/dollar-sign';
	import LockIcon from '@lucide/svelte/icons/lock';
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import { Button } from '$lib/components/ui/button';
	import { formatCompactMoney } from '$lib/money';
	import { dataRoomCompanyText } from './company-text';
	import { daysUntil, humanizedKey, recordAttributes, type CompanyRecord } from './company';

	type Term = { key: string; label: string; value: string };

	let {
		round,
		contact,
		sharedWith,
		locale
	}: { round: CompanyRecord; contact?: string; sharedWith?: string | null; locale: 'ko' | 'en' } = $props();

	const leadingTermKeys = ['preMoney', 'postMoney', 'valuation', 'instrument', 'leadInvestor'];

	const text = $derived(dataRoomCompanyText[locale]);
	const attributes = $derived(new Map(recordAttributes(round, locale)));
	const daysLeft = $derived(round.date ? daysUntil(round.date, new Date()) : 0);
	const committedShare = $derived(
		round.amount && round.committed && round.committed.currency === round.amount.currency && round.amount.amount > 0
			? Math.min(1, round.committed.amount / round.amount.amount)
			: undefined
	);
	const percent = $derived(committedShare === undefined ? '' : new Intl.NumberFormat(locale, { style: 'percent' }).format(committedShare));
	const terms = $derived(raiseTerms());
	const inquiry = $derived(contact ? `mailto:${contact}?subject=${encodeURIComponent(text.raise.inquirySubject(round.title))}` : '');
	const privateNotice = $derived(sharedWith === undefined ? '' : sharedWith ? text.raise.sharedWith(sharedWith) : text.raise.sharedWithInvited);
	const hasProgress = $derived(Boolean(round.amount) || daysLeft > 0 || Boolean(inquiry));

	function labelOf(key: string): string {
		return text.attributeNames[key] ?? humanizedKey(key);
	}

	function raiseTerms(): Term[] {
		const leading = leadingTermKeys.filter((key) => attributes.has(key));
		const remaining = [...attributes.keys()].filter((key) => !leadingTermKeys.includes(key));
		const fromAttributes = [...leading, ...remaining].map((key) => ({ key, label: labelOf(key), value: attributes.get(key) ?? '' }));
		const closing = round.date ? [{ key: 'closes', label: text.raise.closes, value: round.date.replaceAll('-', '.') }] : [];
		return [...fromAttributes, ...closing];
	}
</script>

<section
	class="bg-background grid overflow-hidden rounded-lg border shadow-xs md:grid-cols-[minmax(0,1fr)_20rem] md:grid-rows-[auto_1fr]"
	aria-labelledby="company-raise"
>
	<div class="px-6 pt-6 pb-6 sm:px-8 sm:pt-8 md:pb-0">
		<div class="grid gap-2">
			<p class="text-muted-foreground inline-flex items-center gap-1.5 text-sm font-medium">
				<DollarSignIcon class="size-4" aria-hidden="true" />{text.raise.label}
			</p>
			<h3 id="company-raise" class="text-3xl font-semibold tracking-tight text-balance">{round.title}</h3>
			{#if round.detail}<p class="text-muted-foreground max-w-[60ch] text-pretty">{round.detail}</p>{/if}
		</div>
	</div>

	{#if hasProgress}
		<div class="bg-foreground text-background flex flex-col gap-6 p-6 md:col-start-2 md:row-span-2 md:row-start-1">
			{#if round.amount}
				<div class="grid gap-3">
					{#if committedShare !== undefined}
						<div class="bg-background/15 h-1.5 overflow-hidden rounded-sm" role="img" aria-label={text.raise.share(percent)}>
							<div class="bg-background h-full" style:width={`${committedShare * 100}%`}></div>
						</div>
					{/if}
					<div class="grid gap-0.5 tabular-nums">
						<p class="flex items-baseline justify-between gap-3">
							<span class="text-3xl font-semibold tracking-tight">{formatCompactMoney(round.committed ?? round.amount, locale)}</span>
							{#if round.committed && percent}<span class="text-lg font-semibold">{percent}</span>{/if}
						</p>
						<p class="text-background/60 text-sm">
							{round.committed ? text.raise.committedOfTarget(formatCompactMoney(round.amount, locale)) : text.raise.target}
						</p>
					</div>
				</div>
			{/if}
			{#if daysLeft > 0}
				<div class="border-background/15 grid gap-0.5 border-t pt-5 tabular-nums">
					<p class="text-2xl font-semibold tracking-tight">{text.raise.daysLeft(daysLeft)}</p>
					<p class="text-background/60 text-sm">{text.raise.leftToInvest}</p>
				</div>
			{/if}
			{#if inquiry}
				<div class="grid gap-2 md:mt-auto">
					<Button href={inquiry} variant="secondary" size="lg" class="w-full">{text.raise.inquire}<ArrowUpRightIcon /></Button>
					<p class="text-background/60 truncate text-center text-xs">{contact}</p>
				</div>
			{/if}
		</div>
	{/if}

	{#if terms.length > 0}
		<div class="max-md:border-t px-6 py-6 sm:px-8 md:pt-6 md:pb-8">
			<div class="grid gap-3">
				<h4 class="text-muted-foreground text-xs font-medium">{text.raise.terms}</h4>
				<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-8 gap-y-2.5 text-sm">
					{#each terms as term (term.key)}
						<dt class="text-muted-foreground">{term.label}</dt>
						<dd class="font-medium text-pretty">{term.value}</dd>
					{/each}
				</dl>
			</div>
			{#if privateNotice}<p class="text-muted-foreground mt-6 inline-flex items-center gap-1.5 text-xs"><LockIcon class="size-3.5" aria-hidden="true" />{privateNotice}</p>{/if}
		</div>
	{/if}
</section>
