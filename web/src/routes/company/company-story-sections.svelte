<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import ArrowUpRightIcon from '@lucide/svelte/icons/arrow-up-right';
	import MailIcon from '@lucide/svelte/icons/mail';
	import {
		companyMetricChangeAssessment,
		companyMetricChangePercentage,
		companyMetricLabel,
		companyMetricPeriodLabel,
		companyMetricSeries,
		companyRecordDescription,
		companyRecordTitle,
		formatCompanyMetricValue,
		latestCompanyMetrics,
		type CompanyMetricCurrency,
		type CompanyMetricDisplayCurrency,
		type CompanyShareMetricContext,
		type CompanyShareMetric,
		type CompanyShareNarrative,
		type CompanyShareDocument,
		type CompanyShareRecord
	} from './company-page-model';
	import CompanyMetricAreaChart from './company-metric-area-chart.svelte';

	type CompanyStoryText = {
		highlights: string;
		traction: string;
		latestPeriod: string;
		increase: string;
		businessModel: string;
		customerEvidence: string;
		marketOpportunity: string;
		competitiveAdvantage: string;
		milestones: string;
		roadmap: string;
		funding: string;
		fundingTarget: string;
		useOfFunds: string;
		contact: string;
		noNarrative: string;
		source: string;
		documents: string;
		documentDescription: string;
		documentTypes: Record<string, string>;
		evidenceRoles: Record<string, string>;
		recordCategories: Record<string, string>;
	};

	type CompanyStoryProps = {
		metrics: CompanyShareMetric[];
		primaryMetric?: string;
		metricContexts: Record<string, CompanyShareMetricContext>;
		records: CompanyShareRecord[];
		documents: CompanyShareDocument[];
		narrative: CompanyShareNarrative;
		displayCurrency: CompanyMetricDisplayCurrency;
		localCurrency?: CompanyMetricCurrency;
		language: string;
		text: CompanyStoryText;
		displayCurrencyLabel: string;
		contactEmail?: string;
		onDisplayCurrencyChange: (value: string) => void;
	};

	let {
		metrics, primaryMetric, metricContexts, records, documents, narrative, displayCurrency, localCurrency, language, text,
		displayCurrencyLabel, contactEmail, onDisplayCurrencyChange
	}: CompanyStoryProps = $props();
	const metricGroups = $derived(latestCompanyMetrics(metrics).map((latest) => ({ latest, series: companyMetricSeries(metrics, latest.metric) })));
	const primaryMetricGroup = $derived(primaryMetric ? metricGroups.find((group) => group.latest.metric === primaryMetric) : undefined);
	const secondaryMetricGroups = $derived(primaryMetricGroup ? metricGroups.filter((group) => group.latest.metric !== primaryMetricGroup.latest.metric) : metricGroups);
	const hasBusinessStory = $derived(!!narrative.businessModel || !!narrative.customerEvidence);
	const hasMarketStory = $derived(!!narrative.marketOpportunity || !!narrative.competitiveAdvantage);
	const hasFundingStory = $derived(!!narrative.fundingStage || !!narrative.fundingTarget || !!narrative.useOfFunds);

	function formatChange(series: CompanyShareMetric[]): string | undefined {
		const change = companyMetricChangePercentage(series, displayCurrency);
		if (change === undefined) return undefined;
		const sign = change > 0 ? '+' : '';
		return `${sign}${new Intl.NumberFormat(language, { maximumFractionDigits: 1 }).format(change)}%`;
	}

	function changeClass(series: CompanyShareMetric[]): string {
		const change = companyMetricChangePercentage(series, displayCurrency);
		const assessment = companyMetricChangeAssessment(change, metricContext(series[0]?.metric ?? '').favorableDirection);
		if (assessment === 'favorable') return 'text-blue-600';
		if (assessment === 'unfavorable') return 'text-destructive';
		return 'text-foreground';
	}

	function metricContext(metricName: string): CompanyShareMetricContext {
		return metricContexts[metricName] ?? { labels: {}, descriptions: {}, favorableDirection: 'neutral' };
	}

	function displayMetricLabel(metricName: string): string {
		return metricContext(metricName).labels[language] || companyMetricLabel(metricName);
	}

	function displayMetricDescription(metricName: string): string {
		return metricContext(metricName).descriptions[language] || '';
	}

	function displayMetricRole(metricName: string): string {
		const role = metricContext(metricName).evidenceRole;
		return role ? text.evidenceRoles[role] ?? '' : '';
	}

	function displayRecordCategory(category: string): string {
		return text.recordCategories[category] ?? category;
	}

	function displayDocumentType(documentType: string): string {
		return text.documentTypes[documentType] ?? companyMetricLabel(documentType);
	}

	function formatDate(value: string): string {
		const date = new Date(`${value}T00:00:00`);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(language, { year: 'numeric', month: 'short' }).format(date);
	}

	function formatIssuedDate(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(language, { year: 'numeric', month: 'short', day: 'numeric' }).format(date);
	}
</script>

{#if metricGroups.length > 0}
	<section class="py-14 sm:py-20">
		<div class="flex flex-wrap items-center justify-between gap-4">
			<h2 class="text-2xl font-semibold text-balance">{text.traction}</h2>
			{#if localCurrency}
				<ToggleGroup.Root type="single" value={displayCurrency} onValueChange={onDisplayCurrencyChange} variant="outline" size="sm" aria-label={displayCurrencyLabel}>
					<ToggleGroup.Item value="USD">USD</ToggleGroup.Item>
					<ToggleGroup.Item value="local">{localCurrency}</ToggleGroup.Item>
				</ToggleGroup.Root>
			{/if}
		</div>

		{#if primaryMetricGroup}
			<div class="mt-8 grid gap-10 lg:grid-cols-[minmax(0,1.5fr)_minmax(17rem,0.7fr)]">
				<div class="border-y py-7">
					<div class="flex flex-wrap items-end justify-between gap-4">
						<div>
							{#if displayMetricRole(primaryMetricGroup.latest.metric)}<Badge variant="secondary" class="mb-3">{displayMetricRole(primaryMetricGroup.latest.metric)}</Badge>{/if}
							<p class="text-muted-foreground text-sm font-medium">{displayMetricLabel(primaryMetricGroup.latest.metric)}</p>
							<p class="mt-2 text-4xl font-semibold tracking-tight tabular-nums sm:text-5xl">{formatCompanyMetricValue(primaryMetricGroup.latest, displayCurrency, language)}</p>
							{#if displayMetricDescription(primaryMetricGroup.latest.metric)}<p class="text-muted-foreground mt-3 max-w-[55ch] text-sm leading-6">{displayMetricDescription(primaryMetricGroup.latest.metric)}</p>{/if}
							{#if primaryMetricGroup.latest.source}<p class="text-muted-foreground mt-3 text-xs">{text.source} · {primaryMetricGroup.latest.source}</p>{/if}
						</div>
						<div class="text-right text-sm">
							{#if formatChange(primaryMetricGroup.series)}
								<p class={`font-semibold tabular-nums ${changeClass(primaryMetricGroup.series)}`}>{formatChange(primaryMetricGroup.series)}</p>
								<p class="text-muted-foreground mt-1">{text.increase}</p>
							{:else}
								<p class="text-muted-foreground">{text.latestPeriod}</p>
							{/if}
							<p class="mt-1 font-medium">{companyMetricPeriodLabel(primaryMetricGroup.latest, language)}</p>
						</div>
					</div>

					{#if primaryMetricGroup.series.length > 1}
						<div class="mt-9">
							<CompanyMetricAreaChart
								metrics={primaryMetricGroup.series}
								label={displayMetricLabel(primaryMetricGroup.latest.metric)}
								{displayCurrency}
								{language}
							/>
						</div>
					{/if}
				</div>

				<dl class="border-y">
					{#each secondaryMetricGroups as group}
						<div class="grid gap-2 border-b py-5 last:border-b-0">
							<dt class="text-muted-foreground text-sm font-medium">{displayMetricLabel(group.latest.metric)}</dt>
							{#if displayMetricRole(group.latest.metric)}<dd><Badge variant="secondary">{displayMetricRole(group.latest.metric)}</Badge></dd>{/if}
							<dd class="flex items-end justify-between gap-4">
								<span class="text-2xl font-semibold tabular-nums">{formatCompanyMetricValue(group.latest, displayCurrency, language)}</span>
								<span class="text-right text-xs">
									{#if formatChange(group.series)}<span class={`block font-semibold tabular-nums ${changeClass(group.series)}`}>{formatChange(group.series)}</span>{/if}
									<span class="text-muted-foreground">{companyMetricPeriodLabel(group.latest, language)}</span>
								</span>
							</dd>
							{#if displayMetricDescription(group.latest.metric)}<dd class="text-muted-foreground text-xs leading-5">{displayMetricDescription(group.latest.metric)}</dd>{/if}
							{#if group.latest.source}<dd class="text-muted-foreground text-xs">{text.source} · {group.latest.source}</dd>{/if}
						</div>
					{/each}
				</dl>
			</div>
		{:else}
			<div class="mt-8 grid border-y md:grid-cols-2">
				{#each secondaryMetricGroups as group}
					<article class="grid content-between gap-7 border-b py-7 md:px-7 md:odd:border-r md:odd:pl-0 md:even:pr-0">
						<div>
							<div class="flex items-start justify-between gap-4">
								<div>
									{#if displayMetricRole(group.latest.metric)}<Badge variant="secondary" class="mb-3">{displayMetricRole(group.latest.metric)}</Badge>{/if}
									<h3 class="text-muted-foreground text-sm font-medium">{displayMetricLabel(group.latest.metric)}</h3>
									<p class="mt-2 text-3xl font-semibold tabular-nums">{formatCompanyMetricValue(group.latest, displayCurrency, language)}</p>
								</div>
								<div class="text-right text-xs">
									{#if formatChange(group.series)}<p class={`font-semibold tabular-nums ${changeClass(group.series)}`}>{formatChange(group.series)}</p>{/if}
									<p class="text-muted-foreground mt-1">{companyMetricPeriodLabel(group.latest, language)}</p>
								</div>
							</div>
							{#if displayMetricDescription(group.latest.metric)}<p class="text-muted-foreground mt-4 max-w-[55ch] text-sm leading-6">{displayMetricDescription(group.latest.metric)}</p>{/if}
							{#if group.latest.source}<p class="text-muted-foreground mt-3 text-xs">{text.source} · {group.latest.source}</p>{/if}
						</div>
						{#if group.series.length > 1}
							<div>
								<CompanyMetricAreaChart
									metrics={group.series}
									label={displayMetricLabel(group.latest.metric)}
									{displayCurrency}
									{language}
									compact
								/>
							</div>
						{/if}
					</article>
				{/each}
			</div>
		{/if}
	</section>
{/if}

{#if narrative.highlights.length > 0}
	<section class="py-14 sm:py-20">
		<h2 class="text-2xl font-semibold text-balance">{text.highlights}</h2>
		<ul class="mt-7 grid border-y md:grid-cols-3 md:divide-x">
			{#each narrative.highlights as highlight}
				<li class="grid grid-cols-[0.5rem_1fr] items-start gap-3 border-b py-6 last:border-b-0 md:border-b-0 md:px-7 md:first:pl-0 md:last:pr-0">
					<span class="mt-2.5 size-1.5 rounded-full bg-blue-600" aria-hidden="true"></span>
					<p class="font-medium leading-7 text-pretty">{highlight}</p>
				</li>
			{/each}
		</ul>
	</section>
{/if}

{#if hasBusinessStory}
	<section class="grid gap-10 border-y py-14 sm:py-20 lg:grid-cols-2 lg:gap-16">
		{#if narrative.businessModel}<div><h2 class="text-xl font-semibold">{text.businessModel}</h2><p class="text-muted-foreground mt-5 max-w-[65ch] whitespace-pre-line leading-8 text-pretty">{narrative.businessModel}</p></div>{/if}
		{#if narrative.customerEvidence}<div><h2 class="text-xl font-semibold">{text.customerEvidence}</h2><p class="text-muted-foreground mt-5 max-w-[65ch] whitespace-pre-line leading-8 text-pretty">{narrative.customerEvidence}</p></div>{/if}
	</section>
{/if}

{#if hasMarketStory}
	<section class="grid gap-10 py-14 sm:py-20 lg:grid-cols-[minmax(0,1.4fr)_minmax(17rem,0.6fr)] lg:gap-16">
		{#if narrative.marketOpportunity}<div><h2 class="text-2xl font-semibold text-balance">{text.marketOpportunity}</h2><p class="text-muted-foreground mt-6 max-w-[68ch] whitespace-pre-line text-lg leading-8 text-pretty">{narrative.marketOpportunity}</p></div>{/if}
		{#if narrative.competitiveAdvantage}<div class="border-t pt-6 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-8"><h3 class="font-semibold">{text.competitiveAdvantage}</h3><p class="text-muted-foreground mt-4 whitespace-pre-line leading-7 text-pretty">{narrative.competitiveAdvantage}</p></div>{/if}
	</section>
{/if}

{#if records.length > 0 || narrative.roadmap}
	<section class="grid gap-12 border-t py-14 sm:py-20 lg:grid-cols-[minmax(0,1.25fr)_minmax(17rem,0.75fr)] lg:gap-16">
		{#if records.length > 0}
			<div>
				<h2 class="text-2xl font-semibold">{text.milestones}</h2>
				<ol class="mt-7 grid gap-7">
					{#each records as record}
						<li class="grid grid-cols-[5.5rem_1fr] gap-5">
							<time class="text-muted-foreground text-sm tabular-nums">{record.date ? formatDate(record.date) : record.category}</time>
							<div>
								<div class="flex flex-wrap items-center gap-2"><h3 class="font-semibold">{companyRecordTitle(record, language)}</h3><Badge variant="secondary">{displayRecordCategory(record.category)}</Badge></div>
								{#if companyRecordDescription(record, language)}<p class="text-muted-foreground mt-2 leading-7 text-pretty">{companyRecordDescription(record, language)}</p>{/if}
								{#if Object.keys(record.attributes ?? {}).length > 0}
									<dl class="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm">
										{#each Object.entries(record.attributes ?? {}) as [attributeKey, attributeValue]}
											<div class="flex gap-2"><dt class="text-muted-foreground">{attributeKey}</dt><dd class="font-medium">{attributeValue}</dd></div>
										{/each}
									</dl>
								{/if}
							</div>
						</li>
					{/each}
				</ol>
			</div>
		{/if}
		{#if narrative.roadmap}<div class="border-t pt-7 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-8"><h2 class="text-xl font-semibold">{text.roadmap}</h2><p class="text-muted-foreground mt-5 whitespace-pre-line leading-8 text-pretty">{narrative.roadmap}</p></div>{/if}
	</section>
{/if}

{#if documents.length > 0}
	<section class="border-t py-14 sm:py-20">
		<div class="max-w-2xl"><h2 class="text-2xl font-semibold text-balance">{text.documents}</h2><p class="text-muted-foreground mt-3 leading-7">{text.documentDescription}</p></div>
		<div class="mt-8 divide-y border-y">
			{#each documents as document}
				<article class="grid gap-3 py-6 md:grid-cols-[9rem_1fr] md:gap-7">
					<div class="text-muted-foreground text-sm"><p>{displayDocumentType(document.documentType)}</p>{#if document.issuedAt}<time class="mt-1 block tabular-nums">{formatIssuedDate(document.issuedAt)}</time>{/if}</div>
					<div><h3 class="font-semibold">{document.title}</h3>{#if document.summary}<p class="text-muted-foreground mt-2 max-w-[68ch] leading-7 text-pretty">{document.summary}</p>{/if}</div>
				</article>
			{/each}
		</div>
	</section>
{/if}

{#if hasFundingStory}
	<section class="bg-primary text-primary-foreground my-6 grid gap-10 rounded-xl px-6 py-8 sm:px-10 sm:py-11 lg:grid-cols-[minmax(0,0.75fr)_minmax(0,1.25fr)]">
		<div>
			<p class="text-primary-foreground/65 text-sm font-medium">{text.funding}</p>
			{#if narrative.fundingStage}<h2 class="mt-3 text-3xl font-semibold text-balance">{narrative.fundingStage}</h2>{/if}
			{#if narrative.fundingTarget}<div class="mt-7"><p class="text-primary-foreground/65 text-sm">{text.fundingTarget}</p><p class="mt-1 text-2xl font-semibold tabular-nums">{narrative.fundingTarget}</p></div>{/if}
		</div>
		<div class="flex flex-col justify-between gap-8 border-t border-primary-foreground/20 pt-7 lg:border-t-0 lg:border-l lg:pt-0 lg:pl-9">
			{#if narrative.useOfFunds}<div><h3 class="font-semibold">{text.useOfFunds}</h3><p class="text-primary-foreground/75 mt-4 whitespace-pre-line leading-7 text-pretty">{narrative.useOfFunds}</p></div>{/if}
			{#if contactEmail}<Button variant="secondary" href={`mailto:${contactEmail}`} class="w-fit"><MailIcon data-icon="inline-start" />{text.contact}<ArrowUpRightIcon data-icon="inline-end" /></Button>{/if}
		</div>
	</section>
{/if}
