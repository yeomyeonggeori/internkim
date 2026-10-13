<script lang="ts">
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import MailIcon from '@lucide/svelte/icons/mail';
	import { Badge } from '$lib/components/ui/badge';
	import { formatCompactMoney } from '$lib/money';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import CompanyMetricChart from './company-metric-chart.svelte';
	import { dataRoomCompanyText } from './company-text';
	import {
		changeFromPrevious,
		companyFacts,
		formatMetricValue,
		humanizedKey,
		isEmptyCompany,
		localized,
		metricSeries,
		periodLabel,
		recordAttributes,
		type CompanyMetricSeries,
		type DataRoomCompany
	} from './company';

	let { company }: { company: DataRoomCompany } = $props();

	const locale = $derived(currentLocale.value);
	const text = $derived(dataRoomCompanyText[locale]);
	const profile = $derived(company.profile);
	const brand = $derived(localized(profile.brandName, locale));
	const name = $derived(localized(profile.name, locale));
	const slogan = $derived(localized(profile.slogan, locale));
	const description = $derived(localized(profile.description, locale));
	const facts = $derived(companyFacts(profile, locale));
	const metrics = $derived(metricSeries(company.metrics));

	function changeLabel(series: CompanyMetricSeries): string {
		const change = changeFromPrevious(series);
		if (change === undefined) return '';
		const formatted = new Intl.NumberFormat(locale, { maximumFractionDigits: 1, signDisplay: 'exceptZero' }).format(change);
		return `${formatted}%`;
	}

	function recordDate(date: string | undefined): string {
		return date ? date.slice(0, 7).replace('-', '.') : '';
	}
</script>

{#if isEmptyCompany(company)}
	<div class="bg-background text-muted-foreground rounded-lg border px-4 py-12 text-center text-sm">{text.empty}</div>
{:else}
	<div class="grid gap-8">
		{#if brand || name || slogan || description}
			<header class="grid gap-2">
				{#if brand || name}<p class="text-muted-foreground text-sm">{[brand, name].filter(Boolean).join(' · ')}</p>{/if}
				{#if slogan}<h2 class="text-2xl font-semibold tracking-tight text-balance">{slogan}</h2>{/if}
				{#if description}<p class="text-muted-foreground max-w-[65ch] leading-7 text-pretty">{description}</p>{/if}
				{#if profile.website || profile.email}
					<div class="mt-1 flex flex-wrap gap-x-5 gap-y-1 text-sm">
						{#if profile.website}<a class="inline-flex items-center gap-1.5 hover:underline" href={profile.website} target="_blank" rel="noreferrer"><GlobeIcon class="text-muted-foreground size-4" />{profile.website.replace(/^https?:\/\//, '')}</a>{/if}
						{#if profile.email}<a class="inline-flex items-center gap-1.5 hover:underline" href={`mailto:${profile.email}`}><MailIcon class="text-muted-foreground size-4" />{profile.email}</a>{/if}
					</div>
				{/if}
			</header>
		{/if}

		{#if facts.length > 0}
			<section aria-label={text.overview}>
				<dl class="bg-background grid grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] overflow-hidden rounded-lg border">
					{#each facts as fact (fact.field)}
						<div class="-mt-px -ml-px border-t border-l px-4 py-3">
							<dt class="text-muted-foreground text-xs">{text.fields[fact.field] ?? fact.field}</dt>
							<dd class="mt-1 text-sm font-medium break-words">{fact.value}</dd>
						</div>
					{/each}
				</dl>
			</section>
		{/if}

		{#if metrics.length > 0}
			<section class="grid gap-3" aria-labelledby="company-metrics">
				<h3 id="company-metrics" class="text-sm font-semibold">{text.metrics}</h3>
				<div class="bg-background grid grid-cols-[repeat(auto-fit,minmax(14rem,1fr))] overflow-hidden rounded-lg border">
					{#each metrics as series (series.metric)}
						{@const label = text.metricNames[series.metric] ?? humanizedKey(series.metric)}
						<article class="-mt-px -ml-px grid content-between gap-3 border-t border-l p-4">
							<div>
								<p class="text-muted-foreground text-xs">{label}</p>
								<p class="mt-1 text-2xl font-semibold tracking-tight tabular-nums">{formatMetricValue(series.latest, locale)}</p>
								<p class="text-muted-foreground mt-0.5 text-xs tabular-nums">
									{periodLabel(series.latest, locale)}{#if changeLabel(series)}<span class="text-foreground ml-2 font-medium">{changeLabel(series)}</span>{/if}
								</p>
							</div>
							{#if series.points.length > 1}<CompanyMetricChart {series} {label} {locale} />{/if}
						</article>
					{/each}
				</div>
			</section>
		{/if}

		{#if company.records.length > 0}
			<section class="grid gap-3" aria-labelledby="company-records">
				<h3 id="company-records" class="text-sm font-semibold">{text.records}</h3>
				<ol class="bg-background divide-y rounded-lg border">
					{#each company.records as record, index (index)}
						{@const attributes = recordAttributes(record)}
						<li class="grid grid-cols-[4.5rem_minmax(0,1fr)] gap-4 px-4 py-3">
							<time class="text-muted-foreground pt-px text-sm tabular-nums">{recordDate(record.date)}</time>
							<div class="min-w-0">
								<div class="flex flex-wrap items-center gap-2">
									<p class="font-medium">{record.title}</p>
									<Badge variant="secondary">{text.recordCategories[record.category] ?? humanizedKey(record.category)}</Badge>
								</div>
								{#if record.amount}<p class="mt-1 text-lg font-semibold tracking-tight tabular-nums">{formatCompactMoney(record.amount, locale)}</p>{/if}
								{#if record.detail}<p class="text-muted-foreground mt-1 text-sm leading-6 text-pretty">{record.detail}</p>{/if}
								{#if attributes.length > 0}
									<dl class="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-xs">
										{#each attributes as [key, value] (key)}
											<div class="flex gap-1.5"><dt class="text-muted-foreground">{key}</dt><dd>{value}</dd></div>
										{/each}
									</dl>
								{/if}
							</div>
						</li>
					{/each}
				</ol>
			</section>
		{/if}

		<p class="text-muted-foreground text-xs">{text.scope}</p>
	</div>
{/if}
