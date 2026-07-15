<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Spinner } from '$lib/components/ui/spinner';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Textarea } from '$lib/components/ui/textarea';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import SendIcon from '@lucide/svelte/icons/send';
	import { onMount } from 'svelte';
	import {
		apiErrorMessage,
		fetchCompanyDocuments,
		fetchCompanyMetrics,
		fetchCompanyRecords,
		fetchCompanyShareSettings,
		publishCompanyShare,
		updateCompanyShareSettings
	} from './admin-api';
	import type { AdminPageText, CompanyDocument, CompanyRecord, CompanyShareMetricContext, CompanyShareNarrative, CompanyShareRecordContext, CompanyShareSettings, CompanyShareSettingsUpdate } from './admin-types';

	type CompanyShareSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	const profileFieldNames = [
		'brandName', 'slogan', 'description', 'website', 'foundedDate', 'employeeCount',
		'jurisdiction', 'representative', 'representativeTitle', 'capital', 'fiscalYearEnd', 'email'
	] as const;
	const sessionHourOptions = [12, 24, 72, 168];
	const localizationOptions = ['es', 'fr', 'de', 'it', 'nl', 'pt-BR', 'ja', 'ko', 'zh-CN', 'zh-TW', 'ar', 'hi', 'id', 'vi', 'th', 'tr'];

	let { adminBaseURL, isDeviceReachable, text }: CompanyShareSectionProps = $props();
	let settings = $state<CompanyShareSettings | null>(null);
	let draft = $state<CompanyShareSettingsUpdate>(emptyDraft());
	let metricNames = $state<string[]>([]);
	let records = $state<CompanyRecord[]>([]);
	let documents = $state<CompanyDocument[]>([]);
	let message = $state('');
	let isLoading = $state(true);
	let isSaving = $state(false);
	let isPublishing = $state(false);
	let editingLanguage = $state('en');
	let languageToAdd = $state('');

	onMount(loadSettings);

	function emptyDraft(): CompanyShareSettingsUpdate {
		return { enabled: false, password: '', sessionHours: 24, languages: ['en'], profileFields: [], metricNames: [], primaryMetric: '', metricContexts: {}, recordIDs: [], recordContexts: {}, documentIDs: [], contactEmail: '', showTeamActivity: false, narratives: emptyNarratives(['en']) };
	}

	function emptyMetricContext(languages = draft.languages): CompanyShareMetricContext {
		return { labels: emptyLocalizedValues(languages), descriptions: emptyLocalizedValues(languages), favorableDirection: 'neutral', evidenceRole: '', showSource: false };
	}

	function emptyRecordContext(languages = draft.languages): CompanyShareRecordContext {
		return { titles: emptyLocalizedValues(languages), descriptions: emptyLocalizedValues(languages), attributeKeys: [] };
	}

	function emptyLocalizedValues(languages: string[]): Record<string, string> {
		return Object.fromEntries(languages.map((language) => [language, '']));
	}

	function cloneLocalizedValues(values: Record<string, string> | undefined, languages: string[]): Record<string, string> {
		return Object.fromEntries(languages.map((language) => [language, values?.[language] ?? '']));
	}

	function cloneRecordContexts(recordContexts: CompanyShareSettings['recordContexts'] | undefined, languages: string[]): Record<string, CompanyShareRecordContext> {
		return Object.fromEntries(Object.entries(recordContexts ?? {}).map(([recordID, context]) => [recordID, {
			titles: cloneLocalizedValues(context.titles, languages),
			descriptions: cloneLocalizedValues(context.descriptions, languages),
			attributeKeys: [...(context.attributeKeys ?? [])]
		}]));
	}

	function cloneMetricContexts(metricContexts: CompanyShareSettings['metricContexts'] | undefined, languages: string[]): Record<string, CompanyShareMetricContext> {
		return Object.fromEntries(Object.entries(metricContexts ?? {}).map(([metricName, context]) => [metricName, {
			...emptyMetricContext(languages), ...context,
			labels: cloneLocalizedValues(context.labels, languages),
			descriptions: cloneLocalizedValues(context.descriptions, languages)
		}]));
	}

	function emptyNarrative(): CompanyShareNarrative {
		return { highlights: [], businessModel: '', customerEvidence: '', marketOpportunity: '', competitiveAdvantage: '', roadmap: '', fundingStage: '', fundingTarget: '', useOfFunds: '' };
	}

	function emptyNarratives(languages: string[]): Record<string, CompanyShareNarrative> {
		return Object.fromEntries(languages.map((language) => [language, emptyNarrative()]));
	}

	function cloneNarratives(narratives: CompanyShareSettings['narratives'] | undefined, languages: string[]): Record<string, CompanyShareNarrative> {
		return Object.fromEntries(languages.map((language) => [language, {
			...emptyNarrative(), ...narratives?.[language], highlights: [...(narratives?.[language]?.highlights ?? [])]
		}]));
	}

	async function loadSettings() {
		if (!adminBaseURL) return;
		isLoading = true;
		message = '';
		try {
			const [loadedSettings, metricsResponse, recordsResponse, documentsResponse] = await Promise.all([
				fetchCompanyShareSettings(adminBaseURL, text.companyShare.loadError),
				fetchCompanyMetrics(adminBaseURL, text.companyShare.loadError),
				fetchCompanyRecords(adminBaseURL, text.companyShare.loadError),
				fetchCompanyDocuments(adminBaseURL, text.companyShare.loadError)
			]);
			settings = loadedSettings;
			const languages = loadedSettings.languages?.length ? [...loadedSettings.languages] : ['en'];
			draft = { ...loadedSettings, password: '', languages, profileFields: [...loadedSettings.profileFields], metricNames: [...loadedSettings.metricNames], primaryMetric: loadedSettings.primaryMetric ?? '', metricContexts: cloneMetricContexts(loadedSettings.metricContexts, languages), recordIDs: [...loadedSettings.recordIDs], recordContexts: cloneRecordContexts(loadedSettings.recordContexts, languages), documentIDs: [...(loadedSettings.documentIDs ?? [])], narratives: cloneNarratives(loadedSettings.narratives, languages) };
			editingLanguage = languages.includes(editingLanguage) ? editingLanguage : 'en';
			for (const metricName of draft.metricNames) draft.metricContexts[metricName] ??= emptyMetricContext();
			for (const recordID of draft.recordIDs) draft.recordContexts[recordID] ??= emptyRecordContext();
			metricNames = [...new Set((metricsResponse.metrics ?? []).map((metric) => metric.metric))].sort();
			records = recordsResponse.records ?? [];
			documents = documentsResponse.documents ?? [];
		} catch (error) {
			message = apiErrorMessage(error, text.companyShare.loadError);
		} finally {
			isLoading = false;
		}
	}

	function toggleValue(values: string[], value: string): string[] {
		return values.includes(value) ? values.filter((candidate) => candidate !== value) : [...values, value];
	}

	function toggleProfileField(fieldName: string) {
		draft.profileFields = toggleValue(draft.profileFields, fieldName);
	}

	function toggleMetric(metricName: string) {
		const isRemoving = draft.metricNames.includes(metricName);
		draft.metricNames = toggleValue(draft.metricNames, metricName);
		if (!isRemoving) draft.metricContexts[metricName] = emptyMetricContext();
		if (isRemoving) {
			const remainingContexts = { ...draft.metricContexts };
			delete remainingContexts[metricName];
			draft.metricContexts = remainingContexts;
			if (draft.primaryMetric === metricName) draft.primaryMetric = '';
		}
	}

	function selectPrimaryMetric(value: string) {
		draft.primaryMetric = value === 'balanced' ? '' : value;
	}

	function updateMetricDirection(metricName: string, value: string) {
		const context = draft.metricContexts[metricName];
		if (!context) return;
		if (value !== 'increase' && value !== 'decrease' && value !== 'neutral') return;
		context.favorableDirection = value;
	}

	function updateMetricRole(metricName: string, value: string) {
		const context = draft.metricContexts[metricName];
		if (!context) return;
		if (value === '' || value === 'growth' || value === 'efficiency' || value === 'scale' || value === 'quality' || value === 'reach' || value === 'capital') {
			context.evidenceRole = value;
		}
	}

	function toggleRecord(recordID: string) {
		const isRemoving = draft.recordIDs.includes(recordID);
		draft.recordIDs = toggleValue(draft.recordIDs, recordID);
		if (!isRemoving) draft.recordContexts[recordID] = emptyRecordContext();
		if (isRemoving) {
			const remainingContexts = { ...draft.recordContexts };
			delete remainingContexts[recordID];
			draft.recordContexts = remainingContexts;
		}
	}

	function toggleRecordAttribute(recordID: string, attributeKey: string) {
		const context = draft.recordContexts[recordID];
		if (!context) return;
		context.attributeKeys = toggleValue(context.attributeKeys, attributeKey);
	}

	function toggleDocument(documentID: string) {
		draft.documentIDs = toggleValue(draft.documentIDs, documentID);
	}

	function recordAttributeEntries(record: CompanyRecord): Array<[string, string]> {
		return Object.entries(record.attributes ?? {}).flatMap(([key, value]) => {
			if (typeof value === 'string') return value.trim() ? [[key, value.trim()]] : [];
			if (typeof value === 'number' || typeof value === 'boolean') return [[key, String(value)]];
			return [];
		});
	}

	function updateHighlights(language: string, value: string) {
		draft.narratives[language].highlights = value.split('\n');
	}

	function addLocalization(language: string) {
		if (!language || draft.languages.includes(language)) return;
		draft.languages = [...draft.languages, language];
		draft.narratives[language] = emptyNarrative();
		for (const context of Object.values(draft.metricContexts)) {
			context.labels[language] = '';
			context.descriptions[language] = '';
		}
		for (const context of Object.values(draft.recordContexts)) {
			context.titles[language] = '';
			context.descriptions[language] = '';
		}
		editingLanguage = language;
		languageToAdd = '';
	}

	function removeLocalization(language: string) {
		if (language === 'en') return;
		draft.languages = draft.languages.filter((candidate) => candidate !== language);
		delete draft.narratives[language];
		for (const context of Object.values(draft.metricContexts)) {
			delete context.labels[language];
			delete context.descriptions[language];
		}
		for (const context of Object.values(draft.recordContexts)) {
			delete context.titles[language];
			delete context.descriptions[language];
		}
		editingLanguage = 'en';
	}

	function copyEnglishLocalization() {
		if (editingLanguage === 'en') return;
		const englishNarrative = draft.narratives.en;
		draft.narratives[editingLanguage] = { ...englishNarrative, highlights: [...englishNarrative.highlights] };
		for (const context of Object.values(draft.metricContexts)) {
			context.labels[editingLanguage] = context.labels.en ?? '';
			context.descriptions[editingLanguage] = context.descriptions.en ?? '';
		}
		for (const context of Object.values(draft.recordContexts)) {
			context.titles[editingLanguage] = context.titles.en ?? '';
			context.descriptions[editingLanguage] = context.descriptions.en ?? '';
		}
	}

	function languageLabel(language: string): string {
		try {
			return new Intl.DisplayNames(['en'], { type: 'language' }).of(language) ?? language;
		} catch {
			return language;
		}
	}

	async function saveSettings() {
		isSaving = true;
		message = '';
		try {
			settings = await updateCompanyShareSettings(adminBaseURL, draft, text.companyShare.saveError);
			draft = { ...draft, password: '' };
			message = text.companyShare.saveSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.companyShare.saveError);
		} finally {
			isSaving = false;
		}
	}

	async function publishSnapshot() {
		isPublishing = true;
		message = '';
		try {
			settings = await updateCompanyShareSettings(adminBaseURL, draft, text.companyShare.saveError);
			draft = { ...draft, password: '' };
			settings = await publishCompanyShare(adminBaseURL, text.companyShare.publishError);
			message = text.companyShare.publishSuccess;
		} catch (error) {
			message = apiErrorMessage(error, text.companyShare.publishError);
		} finally {
			isPublishing = false;
		}
	}

	function publicationLabel(): string {
		if (!settings?.publishedAt) return text.companyShare.notPublished;
		const date = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(settings.publishedAt));
		return text.companyShare.published.replace('{date}', date).replace('{revision}', String(settings.publicationRevision));
	}

	function canPublishSnapshot(): boolean {
		if (!draft.enabled) return false;
		if (settings?.hasPassword) return true;
		return draft.password.trim().length >= 8;
	}
</script>

<div class="grid gap-5">
	<Card.Root>
		<Card.Header>
			<div class="flex flex-wrap items-start justify-between gap-3">
				<div>
					<Card.Title>{text.companyShare.title}</Card.Title>
					<Card.Description>{text.companyShare.description}</Card.Description>
				</div>
				<Badge variant={settings?.enabled ? 'default' : 'secondary'}>{settings?.enabled ? text.companyShare.enabled : text.companyShare.notPublished}</Badge>
			</div>
		</Card.Header>
		<Card.Content>
			<Field.Group>
				<Field.Field orientation="horizontal">
					<Field.Content>
						<Field.Title>{text.companyShare.enabled}</Field.Title>
						<Field.Description>{text.companyShare.enabledDescription}</Field.Description>
					</Field.Content>
					<Switch bind:checked={draft.enabled} disabled={isLoading || isSaving} aria-label={text.companyShare.enabled} />
				</Field.Field>
				<div class="grid gap-5 md:grid-cols-2">
					<Field.Field>
						<Field.Label for="company-share-password">{text.companyShare.password}</Field.Label>
						<Input id="company-share-password" type="password" bind:value={draft.password} placeholder={settings?.hasPassword ? text.companyShare.passwordConfigured : text.companyShare.passwordPlaceholder} autocomplete="new-password" disabled={isLoading || isSaving} />
						<Field.Description>{text.companyShare.passwordDescription}</Field.Description>
					</Field.Field>
					<Field.Field>
						<Field.Label for="company-share-session-hours">{text.companyShare.sessionHours}</Field.Label>
						<Select.Root type="single" bind:value={() => String(draft.sessionHours), (value) => draft.sessionHours = Number(value)} disabled={isLoading || isSaving}>
							<Select.Trigger id="company-share-session-hours" class="w-full">{draft.sessionHours}h</Select.Trigger>
							<Select.Content>
								{#each sessionHourOptions as hours}
									<Select.Item value={String(hours)} label={`${hours}h`}>{hours}h</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</Field.Field>
				</div>
				<Field.Field>
					<Field.Label for="company-share-contact">{text.companyShare.contactEmail}</Field.Label>
					<Input id="company-share-contact" type="email" bind:value={draft.contactEmail} placeholder={text.companyShare.contactEmailPlaceholder} autocomplete="email" disabled={isLoading || isSaving} />
				</Field.Field>
			</Field.Group>
		</Card.Content>
	</Card.Root>

	<Card.Root>
		<Card.Header>
			<Card.Title>{text.companyShare.profileFields}</Card.Title>
			<Card.Description>{text.companyShare.profileFieldsDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
			<Field.Field orientation="horizontal" class="sm:col-span-2 lg:col-span-3">
				<Checkbox bind:checked={draft.showTeamActivity} disabled={isLoading || isSaving} id="company-share-team-activity" />
				<Field.Content>
					<Field.Label for="company-share-team-activity">{text.companyShare.teamActivity}</Field.Label>
					<Field.Description>{text.companyShare.teamActivityDescription}</Field.Description>
				</Field.Content>
			</Field.Field>
			{#each profileFieldNames as fieldName}
				<Field.Field orientation="horizontal">
					<Checkbox checked={draft.profileFields.includes(fieldName)} onclick={() => toggleProfileField(fieldName)} disabled={isLoading || isSaving} id={`company-share-field-${fieldName}`} />
					<Field.Label for={`company-share-field-${fieldName}`}>{text.companyShare.fieldLabels[fieldName]}</Field.Label>
				</Field.Field>
			{/each}
		</Card.Content>
	</Card.Root>

	<Card.Root>
		<Card.Header>
			<Card.Title>{text.companyShare.localization}</Card.Title>
			<Card.Description>{text.companyShare.localizationDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="grid gap-5">
			<Tabs.Root value={editingLanguage} onValueChange={(value) => editingLanguage = value}>
				<Tabs.List class="h-auto flex-wrap justify-start">
					{#each draft.languages as language}
						<Tabs.Trigger value={language}>{languageLabel(language)}{#if language === 'en'} <span class="text-muted-foreground ml-1 text-xs">{text.companyShare.defaultLanguage}</span>{/if}</Tabs.Trigger>
					{/each}
				</Tabs.List>
			</Tabs.Root>
			<div class="flex flex-wrap items-end gap-3">
				<Field.Field class="min-w-56 flex-1 sm:max-w-xs">
					<Field.Label for="company-share-add-language">{text.companyShare.addLocalization}</Field.Label>
					<Select.Root type="single" value={languageToAdd || 'none'} onValueChange={(value) => languageToAdd = value === 'none' ? '' : value} disabled={isLoading || isSaving}>
						<Select.Trigger id="company-share-add-language" class="w-full">{languageToAdd ? languageLabel(languageToAdd) : text.companyShare.chooseLanguage}</Select.Trigger>
						<Select.Content>
							<Select.Item value="none" label={text.companyShare.chooseLanguage}>{text.companyShare.chooseLanguage}</Select.Item>
							{#each localizationOptions.filter((language) => !draft.languages.includes(language)) as language}
								<Select.Item value={language} label={languageLabel(language)}>{languageLabel(language)}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</Field.Field>
				<Button variant="outline" onclick={() => addLocalization(languageToAdd)} disabled={!languageToAdd || isLoading || isSaving}>{text.companyShare.add}</Button>
				{#if editingLanguage !== 'en'}
					<Button variant="outline" onclick={copyEnglishLocalization} disabled={isLoading || isSaving}>{text.companyShare.copyEnglish}</Button>
					<Button variant="ghost" onclick={() => removeLocalization(editingLanguage)} disabled={isLoading || isSaving}>{text.companyShare.removeLocalization}</Button>
				{/if}
			</div>
			<p class="text-muted-foreground text-sm">{text.companyShare.editingLanguage.replace('{language}', languageLabel(editingLanguage))}</p>
		</Card.Content>
	</Card.Root>

	<Card.Root>
		<Card.Header>
			<Card.Title>{text.companyShare.narrative}</Card.Title>
			<Card.Description>{text.companyShare.narrativeDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			{@const narrative = draft.narratives[editingLanguage]}
			{#if narrative}
				<Field.Group>
					<Field.Field>
						<Field.Label for={`company-share-highlights-${editingLanguage}`}>{text.companyShare.narrativeFields.highlights}</Field.Label>
						<Textarea id={`company-share-highlights-${editingLanguage}`} value={narrative.highlights.join('\n')} oninput={(event) => updateHighlights(editingLanguage, event.currentTarget.value)} class="min-h-28 resize-y" placeholder={text.companyShare.highlightsPlaceholder} disabled={isLoading || isSaving} />
						<Field.Description>{text.companyShare.highlightsDescription}</Field.Description>
					</Field.Field>
					<div class="grid gap-5 lg:grid-cols-2">
						<Field.Field><Field.Label for={`company-share-business-${editingLanguage}`}>{text.companyShare.narrativeFields.businessModel}</Field.Label><Textarea id={`company-share-business-${editingLanguage}`} bind:value={narrative.businessModel} class="min-h-32 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						<Field.Field><Field.Label for={`company-share-customers-${editingLanguage}`}>{text.companyShare.narrativeFields.customerEvidence}</Field.Label><Textarea id={`company-share-customers-${editingLanguage}`} bind:value={narrative.customerEvidence} class="min-h-32 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						<Field.Field><Field.Label for={`company-share-market-${editingLanguage}`}>{text.companyShare.narrativeFields.marketOpportunity}</Field.Label><Textarea id={`company-share-market-${editingLanguage}`} bind:value={narrative.marketOpportunity} class="min-h-32 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						<Field.Field><Field.Label for={`company-share-advantage-${editingLanguage}`}>{text.companyShare.narrativeFields.competitiveAdvantage}</Field.Label><Textarea id={`company-share-advantage-${editingLanguage}`} bind:value={narrative.competitiveAdvantage} class="min-h-32 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						<Field.Field><Field.Label for={`company-share-roadmap-${editingLanguage}`}>{text.companyShare.narrativeFields.roadmap}</Field.Label><Textarea id={`company-share-roadmap-${editingLanguage}`} bind:value={narrative.roadmap} class="min-h-32 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						<div class="grid gap-5">
							<div class="grid gap-5 sm:grid-cols-2">
								<Field.Field><Field.Label for={`company-share-stage-${editingLanguage}`}>{text.companyShare.narrativeFields.fundingStage}</Field.Label><Input id={`company-share-stage-${editingLanguage}`} bind:value={narrative.fundingStage} disabled={isLoading || isSaving} /></Field.Field>
								<Field.Field><Field.Label for={`company-share-target-${editingLanguage}`}>{text.companyShare.narrativeFields.fundingTarget}</Field.Label><Input id={`company-share-target-${editingLanguage}`} bind:value={narrative.fundingTarget} disabled={isLoading || isSaving} /></Field.Field>
							</div>
							<Field.Field><Field.Label for={`company-share-use-${editingLanguage}`}>{text.companyShare.narrativeFields.useOfFunds}</Field.Label><Textarea id={`company-share-use-${editingLanguage}`} bind:value={narrative.useOfFunds} class="min-h-24 resize-y" disabled={isLoading || isSaving} /></Field.Field>
						</div>
					</div>
				</Field.Group>
			{/if}
		</Card.Content>
	</Card.Root>

	<div class="grid gap-5 lg:grid-cols-2">
		<Card.Root>
			<Card.Header>
				<Card.Title>{text.companyShare.metrics}</Card.Title>
				<Card.Description>{text.companyShare.metricsDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="grid max-h-72 gap-3 overflow-y-auto">
				{#if metricNames.length === 0}
					<p class="text-muted-foreground text-sm">{text.companyShare.emptyMetrics}</p>
				{:else}
					{#each metricNames as metricName}
						<Field.Field orientation="horizontal">
							<Checkbox checked={draft.metricNames.includes(metricName)} onclick={() => toggleMetric(metricName)} disabled={isLoading || isSaving} id={`company-share-metric-${metricName}`} />
							<Field.Label for={`company-share-metric-${metricName}`}>{metricName}</Field.Label>
						</Field.Field>
					{/each}
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header>
				<Card.Title>{text.companyShare.records}</Card.Title>
				<Card.Description>{text.companyShare.recordsDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="grid max-h-72 gap-3 overflow-y-auto">
				{#if records.length === 0}
					<p class="text-muted-foreground text-sm">{text.companyShare.emptyRecords}</p>
				{:else}
					{#each records as record (record.id)}
						<Field.Field orientation="horizontal">
							<Checkbox checked={draft.recordIDs.includes(record.id)} onclick={() => toggleRecord(record.id)} disabled={isLoading || isSaving} id={`company-share-record-${record.id}`} />
							<Field.Content>
								<Field.Label for={`company-share-record-${record.id}`}>{record.title}</Field.Label>
								<Field.Description>{[record.date, record.category].filter(Boolean).join(' · ')}</Field.Description>
							</Field.Content>
						</Field.Field>
					{/each}
				{/if}
			</Card.Content>
		</Card.Root>
	</div>

	<Card.Root>
		<Card.Header>
			<Card.Title>{text.companyShare.documents}</Card.Title>
			<Card.Description>{text.companyShare.documentsDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="grid max-h-80 gap-3 overflow-y-auto sm:grid-cols-2">
			{#if documents.length === 0}
				<p class="text-muted-foreground text-sm">{text.companyShare.emptyDocuments}</p>
			{:else}
				{#each documents as document (document.id)}
					<Field.Field orientation="horizontal" class="items-start">
						<Checkbox checked={draft.documentIDs.includes(document.id)} onclick={() => toggleDocument(document.id)} disabled={isLoading || isSaving} id={`company-share-document-${document.id}`} />
						<Field.Content>
							<Field.Label for={`company-share-document-${document.id}`}>{document.title}</Field.Label>
							<Field.Description>{[document.documentType, document.language, document.issuedAt?.slice(0, 10)].filter(Boolean).join(' · ')}</Field.Description>
							{#if document.summary}<p class="text-muted-foreground mt-1 line-clamp-2 text-xs leading-5">{document.summary}</p>{/if}
						</Field.Content>
					</Field.Field>
				{/each}
			{/if}
		</Card.Content>
	</Card.Root>

	{#if draft.recordIDs.length > 0}
		<Card.Root>
			<Card.Header>
				<Card.Title>{text.companyShare.recordPresentation}</Card.Title>
				<Card.Description>{text.companyShare.recordPresentationDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="grid gap-4">
				{#each draft.recordIDs as recordID}
					{@const record = records.find((candidate) => candidate.id === recordID)}
					{@const context = draft.recordContexts[recordID]}
					{#if record && context}
						{@const attributeEntries = recordAttributeEntries(record)}
						<section class="grid gap-5 rounded-lg border p-4">
							<div><h3 class="font-semibold">{record.title}</h3><p class="text-muted-foreground mt-1 text-xs">{[record.date, record.category].filter(Boolean).join(' · ')}</p></div>
							<div class="grid gap-5 md:grid-cols-2">
								<Field.Field><Field.Label for={`company-share-record-title-${editingLanguage}-${recordID}`}>{text.companyShare.recordTitle}</Field.Label><Input id={`company-share-record-title-${editingLanguage}-${recordID}`} bind:value={context.titles[editingLanguage]} placeholder={record.title} disabled={isLoading || isSaving} /></Field.Field>
								<Field.Field><Field.Label for={`company-share-record-description-${editingLanguage}-${recordID}`}>{text.companyShare.recordDescription}</Field.Label><Textarea id={`company-share-record-description-${editingLanguage}-${recordID}`} bind:value={context.descriptions[editingLanguage]} placeholder={record.detail ?? ''} class="min-h-20 resize-y" disabled={isLoading || isSaving} /></Field.Field>
							</div>
							<div>
								<p class="text-sm font-medium">{text.companyShare.recordAttributes}</p>
								{#if attributeEntries.length === 0}
									<p class="text-muted-foreground mt-2 text-sm">{text.companyShare.noRecordAttributes}</p>
								{:else}
									<div class="mt-3 flex flex-wrap gap-2">
										{#each attributeEntries as [attributeKey, attributeValue]}
											<label class="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
												<Checkbox checked={context.attributeKeys.includes(attributeKey)} onclick={() => toggleRecordAttribute(recordID, attributeKey)} disabled={isLoading || isSaving} />
												<span><span class="font-medium">{attributeKey}</span> <span class="text-muted-foreground">{attributeValue}</span></span>
											</label>
										{/each}
									</div>
								{/if}
							</div>
						</section>
					{/if}
				{/each}
			</Card.Content>
		</Card.Root>
	{/if}

	{#if draft.metricNames.length > 0}
		<Card.Root>
			<Card.Header>
				<Card.Title>{text.companyShare.metricPresentation}</Card.Title>
				<Card.Description>{text.companyShare.metricPresentationDescription}</Card.Description>
			</Card.Header>
			<Card.Content class="grid gap-6">
				<Field.Field class="max-w-md">
					<Field.Label for="company-share-primary-metric">{text.companyShare.primaryMetric}</Field.Label>
					<Select.Root type="single" value={draft.primaryMetric || 'balanced'} onValueChange={selectPrimaryMetric} disabled={isLoading || isSaving}>
						<Select.Trigger id="company-share-primary-metric" class="w-full">{draft.primaryMetric || text.companyShare.balancedMetrics}</Select.Trigger>
						<Select.Content>
							<Select.Item value="balanced" label={text.companyShare.balancedMetrics}>{text.companyShare.balancedMetrics}</Select.Item>
							{#each draft.metricNames as metricName}<Select.Item value={metricName} label={metricName}>{metricName}</Select.Item>{/each}
						</Select.Content>
					</Select.Root>
					<Field.Description>{text.companyShare.primaryMetricDescription}</Field.Description>
				</Field.Field>

				<div class="grid gap-4">
					{#each draft.metricNames as metricName}
						{@const context = draft.metricContexts[metricName]}
						{#if context}
							<div class="grid gap-5 rounded-lg border p-4">
								<div class="flex flex-wrap items-end justify-between gap-4">
									<p class="font-semibold">{metricName}</p>
									<div class="grid w-full gap-4 sm:w-auto sm:grid-cols-2">
										<Field.Field class="sm:w-48">
											<Field.Label for={`company-share-role-${metricName}`}>{text.companyShare.metricRole}</Field.Label>
											<Select.Root type="single" value={context.evidenceRole || 'none'} onValueChange={(value) => updateMetricRole(metricName, value === 'none' ? '' : value)} disabled={isLoading || isSaving}>
												<Select.Trigger id={`company-share-role-${metricName}`} class="w-full">{text.companyShare.roleLabels[context.evidenceRole || 'none']}</Select.Trigger>
												<Select.Content>
													{#each Object.entries(text.companyShare.roleLabels) as [value, label]}<Select.Item {value} {label}>{label}</Select.Item>{/each}
												</Select.Content>
											</Select.Root>
										</Field.Field>
										<Field.Field class="sm:w-56">
											<Field.Label for={`company-share-direction-${metricName}`}>{text.companyShare.favorableDirection}</Field.Label>
											<Select.Root type="single" value={context.favorableDirection} onValueChange={(value) => updateMetricDirection(metricName, value)} disabled={isLoading || isSaving}>
												<Select.Trigger id={`company-share-direction-${metricName}`} class="w-full">{text.companyShare.directionLabels[context.favorableDirection]}</Select.Trigger>
												<Select.Content>
													<Select.Item value="neutral" label={text.companyShare.directionLabels.neutral}>{text.companyShare.directionLabels.neutral}</Select.Item>
													<Select.Item value="increase" label={text.companyShare.directionLabels.increase}>{text.companyShare.directionLabels.increase}</Select.Item>
													<Select.Item value="decrease" label={text.companyShare.directionLabels.decrease}>{text.companyShare.directionLabels.decrease}</Select.Item>
												</Select.Content>
											</Select.Root>
										</Field.Field>
									</div>
								</div>
								<div class="grid gap-5 md:grid-cols-2">
									<Field.Field><Field.Label for={`company-share-metric-label-${editingLanguage}-${metricName}`}>{text.companyShare.metricLabel}</Field.Label><Input id={`company-share-metric-label-${editingLanguage}-${metricName}`} bind:value={context.labels[editingLanguage]} placeholder={metricName} disabled={isLoading || isSaving} /></Field.Field>
									<Field.Field><Field.Label for={`company-share-metric-description-${editingLanguage}-${metricName}`}>{text.companyShare.metricDescription}</Field.Label><Textarea id={`company-share-metric-description-${editingLanguage}-${metricName}`} bind:value={context.descriptions[editingLanguage]} class="min-h-20 resize-y" disabled={isLoading || isSaving} /></Field.Field>
								</div>
								<Field.Field orientation="horizontal">
									<Checkbox bind:checked={context.showSource} disabled={isLoading || isSaving} id={`company-share-metric-source-${metricName}`} />
									<Field.Content><Field.Label for={`company-share-metric-source-${metricName}`}>{text.companyShare.showMetricSource}</Field.Label><Field.Description>{text.companyShare.showMetricSourceDescription}</Field.Description></Field.Content>
								</Field.Field>
							</div>
						{/if}
					{/each}
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	<Card.Root>
		<Card.Header>
			<Card.Title>{publicationLabel()}</Card.Title>
			<Card.Description>{text.companyShare.description}</Card.Description>
		</Card.Header>
		<Card.Footer class="flex flex-wrap justify-between gap-3">
			<div class="flex flex-wrap gap-2">
				<Button onclick={saveSettings} disabled={!isDeviceReachable || isLoading || isSaving || isPublishing}>
					{#if isSaving}<Spinner data-icon="inline-start" />{/if}
					{text.companyShare.save}
				</Button>
				<Button variant="outline" onclick={publishSnapshot} disabled={!isDeviceReachable || !canPublishSnapshot() || isLoading || isSaving || isPublishing}>
					{#if isPublishing}<Spinner data-icon="inline-start" />{:else}<SendIcon data-icon="inline-start" />{/if}
					{text.companyShare.publish}
				</Button>
			</div>
			<Button variant="ghost" href="/company/" target="_blank" rel="noreferrer">
				<ExternalLinkIcon data-icon="inline-start" />
				{text.companyShare.openPage}
			</Button>
		</Card.Footer>
	</Card.Root>

	{#if message}
		<p role="status" class="text-muted-foreground text-sm">{message}</p>
	{/if}
</div>
