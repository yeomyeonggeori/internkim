<script lang="ts">
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import FolderIcon from '@lucide/svelte/icons/folder';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PhoneIcon from '@lucide/svelte/icons/phone';
	import { formatCompactMoney } from '$lib/money';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { cn } from '$lib/utils';
	import CompanyActivity from './company-activity.svelte';
	import CompanyMetricCard from './company-metric-card.svelte';
	import CompanyOwnership from './company-ownership.svelte';
	import CompanyProof from './company-proof.svelte';
	import CompanyRaise from './company-raise.svelte';
	import CompanyRecordList from './company-record-list.svelte';
	import { displayCurrency } from '$lib/currency/display-currency.svelte';
	import { companyIn, loadExchangeRates, recordedCurrencies, type ExchangeRates } from './company-currency';
	import { dataRoomCompanyText } from './company-text';
	import {
		changeFromPrevious,
		projectedChange,
		completedRecords,
		roundBeingRaised,
		companyFactGroups,
		formatMetricValue,
		fundingCategories,
		headcount,
		headcountMetric,
		headlineSeries,
		isEmptyCompany,
		localized,
		metricSeries,
		periodKind,
		periodLabel,
		proofGroups,
		recordsIn,
		milestoneRecords,
		newestFirst,
		totalRaised,
		yearsOperating,
		type CompanyMetricSeries,
		type MetricChange,
		type DataRoomCompany
	} from './company';

	type Folder = { code: string; name: string; documentCount: number };
	type Highlight = { key: string; label: string; value: string; note: string };

	let {
		company: recordedCompany,
		folders = [],
		sharedWith,
		onOpenFolder
	}: { company: DataRoomCompany; folders?: Folder[]; sharedWith?: string | null; onOpenFolder?: (code: string) => void } = $props();

	let exchangeRates = $state<ExchangeRates>(new Map());

	const locale = $derived(currentLocale.value);
	const text = $derived(dataRoomCompanyText[locale]);
	const currency = $derived(displayCurrency.value);
	const company = $derived(currency ? companyIn(recordedCompany, currency, exchangeRates) : recordedCompany);
	const conversionDate = $derived([...exchangeRates.values()].map((rate) => rate.asOf).toSorted().at(-1));
	const conversionNotice = $derived(currency && conversionDate ? text.currency.converted(currency, conversionDate) : '');

	$effect(() => {
		if (!currency) return;
		const target = currency;
		void loadExchangeRates(recordedCurrencies(recordedCompany), target).then((rates) => {
			if (displayCurrency.value === target) exchangeRates = rates;
		});
	});
	const profile = $derived(company.profile);
	const slogan = $derived(localized(profile.slogan, locale));
	const description = $derived(localized(profile.description, locale));
	const factGroups = $derived(companyFactGroups(profile, locale));
	const people = $derived(headcount(company));
	const metrics = $derived(
		metricSeries(company.metrics)
			.filter((series) => people === undefined || series.metric !== headcountMetric)
			.toSorted((left, right) => Number(right.latest.amount !== undefined) - Number(left.latest.amount !== undefined))
	);
	const metricColumns = $derived(metrics.length % 3 === 0 || metrics.length > 4 ? 3 : Math.min(metrics.length, 2));
	const fundingRecords = $derived(completedRecords(recordsIn(company.records, fundingCategories)).toSorted(newestFirst));
	const raising = $derived(roundBeingRaised(company.records));
	const proof = $derived(proofGroups(company.records));
	const timeline = $derived(milestoneRecords(company.records));
	const highlights = $derived(companyHighlights());
	const documentTotal = $derived(folders.reduce((sum, folder) => sum + folder.documentCount, 0));
	const hasIntroduction = $derived(
		Boolean(slogan || description || profile.website || profile.email || profile.phone || raising) || highlights.length > 0
	);

	function companyHighlights(): Highlight[] {
		const years = yearsOperating(profile.foundedDate, new Date());
		const raised = totalRaised(company.records);
		const revenue = metrics.length === 0 ? headlineSeries(metricSeries(company.metrics)) : undefined;
		const candidates: (Highlight | undefined)[] = [
			profile.foundedDate && years
				? { key: 'founded', label: text.highlights.founded, value: profile.foundedDate.slice(0, 4), note: text.yearsOperating(years) }
				: undefined,
			people === undefined ? undefined : { key: 'employees', label: text.highlights.employees, value: text.people(people), note: '' },
			raised
				? { key: 'raised', label: text.highlights.raised, value: formatCompactMoney(raised, locale), note: text.rounds(completedRecords(recordsIn(company.records, ['funding'])).length) }
				: undefined,
			revenue
				? { key: 'revenue', label: `${text.highlights.revenue} · ${periodLabel(revenue.latest, locale)}`, value: formatMetricValue(revenue.latest, locale), note: changeNote(changeFromPrevious(revenue), revenue) }
				: undefined
		];
		return candidates.filter((highlight) => highlight !== undefined);
	}

	function changeNote(change: MetricChange | undefined, series: CompanyMetricSeries): string {
		if (!change) return '';
		if (change.kind === 'turnedPositive') return text.turnedPositive;
		if (change.kind === 'turnedNegative') return text.turnedNegative;
		const percent = new Intl.NumberFormat(locale, { maximumFractionDigits: 1, signDisplay: 'exceptZero' }).format(change.percent);
		return text.change(`${percent}%`, periodKind(series.latest));
	}

	function lastRowSpan(count: number, columns: number): number {
		const remainder = count % columns;
		return remainder === 0 ? 1 : columns - remainder + 1;
	}
</script>

{#if isEmptyCompany(company)}
	<div class="bg-background text-muted-foreground rounded-lg border px-4 py-12 text-center text-sm">{text.empty}</div>
{:else}
	<div class="grid gap-10 break-keep">
		{#if conversionNotice}<p class="text-muted-foreground -mb-6 text-xs">{conversionNotice}</p>{/if}
		{#if hasIntroduction}
			<header class="grid gap-5">
				{#if raising}<CompanyRaise round={raising} contact={profile.email} {sharedWith} {locale} />{/if}
				{#if slogan || description}
					<div class="grid max-w-[68ch] gap-2">
						{#if slogan}<p class="text-lg font-medium text-pretty">{slogan}</p>{/if}
						{#if description}<p class="text-muted-foreground leading-relaxed text-pretty">{description}</p>{/if}
					</div>
				{/if}
				{#if profile.website || profile.email || profile.phone}
					<div class="flex flex-wrap gap-x-5 gap-y-1 text-sm">
						{#if profile.website}<a class="inline-flex items-center gap-1.5 hover:underline" href={profile.website} target="_blank" rel="noreferrer"><GlobeIcon class="text-muted-foreground size-4" />{profile.website.replace(/^https?:\/\//, '')}</a>{/if}
						{#if profile.email}<a class="inline-flex items-center gap-1.5 hover:underline" href={`mailto:${profile.email}`}><MailIcon class="text-muted-foreground size-4" />{profile.email}</a>{/if}
						{#if profile.phone}<a class="inline-flex items-center gap-1.5 tabular-nums hover:underline" href={`tel:${profile.phone}`}><PhoneIcon class="text-muted-foreground size-4" />{profile.phone}</a>{/if}
					</div>
				{/if}

				{#if highlights.length > 0}
					<dl
						class="bg-border grid grid-cols-2 gap-px overflow-hidden rounded-lg border md:grid-cols-(--highlight-columns)"
						style:--highlight-columns={`repeat(${highlights.length}, minmax(0, 1fr))`}
					>
						{#each highlights as highlight, index (highlight.key)}
							<div class={cn('bg-background grid content-start gap-1 px-5 py-4', index === highlights.length - 1 && highlights.length % 2 === 1 && 'max-md:col-span-2')}>
								<dt class="text-muted-foreground text-xs">{highlight.label}</dt>
								<dd class="text-2xl font-semibold tracking-tight tabular-nums">{highlight.value}</dd>
								{#if highlight.note}<dd class="text-muted-foreground text-xs tabular-nums">{highlight.note}</dd>{/if}
							</div>
						{/each}
					</dl>
				{/if}
			</header>
		{/if}

		{#if metrics.length > 0}
			<section class="grid gap-3" aria-labelledby="company-metrics">
				<h3 id="company-metrics" class="text-lg font-semibold">{text.metrics}</h3>
				<div
					class="bg-border grid gap-px overflow-hidden rounded-lg border sm:grid-cols-(--metric-columns)"
					style:--metric-columns={`repeat(${metricColumns}, minmax(0, 1fr))`}
				>
					{#each metrics as series, index (series.metric)}
						<CompanyMetricCard
							{series}
							{locale}
							note={changeNote(changeFromPrevious(series), series)}
							projectionNote={changeNote(projectedChange(series), series)}
							span={index === metrics.length - 1 ? lastRowSpan(metrics.length, metricColumns) : 1}
						/>
					{/each}
				</div>
			</section>
		{/if}

		{#if fundingRecords.length > 0}
			<CompanyRecordList id="company-funding" title={text.funding} records={fundingRecords} plainCategory="funding" {locale} />
		{/if}
		{#if company.ownership}<CompanyOwnership ownership={company.ownership} {locale} />{/if}
		{#if timeline.length > 0}
			<CompanyRecordList id="company-timeline" title={text.records} records={timeline} plainCategory="history" {locale} />
		{/if}
		{#if proof.length > 0}<CompanyProof groups={proof} {locale} />{/if}
		{#if company.activity}<CompanyActivity activity={company.activity} {locale} />{/if}

		{#if factGroups.length > 0}
			<section class="grid gap-3" aria-labelledby="company-legal">
				<h3 id="company-legal" class="text-lg font-semibold">{text.legal}</h3>
				<div class="bg-border grid gap-px overflow-hidden rounded-lg border sm:grid-cols-(--fact-columns)" style:--fact-columns={`repeat(${factGroups.length}, minmax(0, 1fr))`}>
					{#each factGroups as group (group.group)}
						<article class="bg-background grid content-start gap-3 p-5">
							<h4 class="text-sm font-semibold">{text.factGroups[group.group]}</h4>
							<dl class="grid gap-2">
								{#each group.facts as fact (fact.field)}
									<div class="grid grid-cols-[auto_minmax(0,1fr)] gap-3 text-sm">
										<dt class="text-muted-foreground">{text.fields[fact.field] ?? fact.field}</dt>
										<dd class="text-right break-words tabular-nums">{fact.value}</dd>
									</div>
								{/each}
							</dl>
						</article>
					{/each}
				</div>
			</section>
		{/if}

		{#if documentTotal > 0}
			<section class="grid gap-3" aria-labelledby="company-contents">
				<div class="flex items-baseline justify-between gap-3">
					<h3 id="company-contents" class="text-lg font-semibold">{text.contents}</h3>
					<p class="text-muted-foreground text-xs tabular-nums">{text.documents(documentTotal)}</p>
				</div>
				<ul class="bg-border grid gap-px overflow-hidden rounded-lg border sm:grid-cols-2">
					{#each folders as folder, index (folder.code)}
						<li class={cn('bg-background', index === folders.length - 1 && folders.length % 2 === 1 && 'sm:col-span-2')}>
							<button
								type="button"
								class="hover:bg-accent/60 flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition-colors disabled:cursor-default disabled:hover:bg-transparent"
								disabled={!onOpenFolder}
								onclick={() => onOpenFolder?.(folder.code)}
							>
								<FolderIcon class="text-muted-foreground size-4 shrink-0" />
								<span class="min-w-0 flex-1 truncate">{folder.name}</span>
								<span class="text-muted-foreground text-xs tabular-nums">{folder.documentCount}</span>
								{#if onOpenFolder}<ChevronRightIcon class="text-muted-foreground size-4" />{/if}
							</button>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<p class="text-muted-foreground border-t pt-4 text-xs leading-5">{text.scope}</p>
	</div>
{/if}
