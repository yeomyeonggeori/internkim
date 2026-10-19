<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { formatCompactMoney } from '$lib/money';
	import { periodLabel } from '$lib/company-period';
	import { dataRoomCompanyText } from './company-text';
	import { humanizedKey, isInProgress, recordAttributes, type CompanyRecord } from './company';

	let {
		id,
		title,
		records,
		plainCategory,
		locale
	}: { id: string; title: string; records: CompanyRecord[]; plainCategory: string; locale: 'ko' | 'en' } = $props();

	const text = $derived(dataRoomCompanyText[locale]);

	function monthOf(date: string | undefined): string {
		return date ? periodLabel(date, locale) : text.undated;
	}
</script>

<section class="grid gap-3" aria-labelledby={id}>
	<h3 {id} class="text-lg font-semibold">{title}</h3>
	<ol class="bg-background divide-y overflow-hidden rounded-lg border">
		{#each records as record, index (index)}
			{@const attributes = recordAttributes(record, locale)}
			<li class="grid grid-cols-[4.5rem_minmax(0,1fr)_auto] items-baseline gap-x-4 px-4 py-3">
				<time class="text-muted-foreground text-sm tabular-nums">{monthOf(record.date)}</time>
				<div class="min-w-0">
					<div class="flex flex-wrap items-center gap-2">
						<p class="font-medium">{record.title}</p>
						{#if record.category !== plainCategory}<Badge variant="secondary">{text.recordCategories[record.category] ?? humanizedKey(record.category)}</Badge>{/if}
						{#if isInProgress(record)}<Badge variant="outline">{text.inProgress}</Badge>{/if}
					</div>
					{#if record.detail}<p class="text-muted-foreground mt-0.5 text-sm text-pretty">{record.detail}</p>{/if}
					{#if attributes.length > 0}
						<dl class="mt-1 flex flex-wrap gap-x-4 gap-y-0.5 text-xs">
							{#each attributes as [key, value] (key)}
								<div class="flex gap-1.5"><dt class="text-muted-foreground">{text.attributeNames[key] ?? humanizedKey(key)}</dt><dd>{value}</dd></div>
							{/each}
						</dl>
					{/if}
				</div>
				<p class="text-right font-semibold tabular-nums">{record.amount ? formatCompactMoney(record.amount, locale) : ''}</p>
			</li>
		{/each}
	</ol>
</section>
