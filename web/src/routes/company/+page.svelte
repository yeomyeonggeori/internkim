<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import { Spinner } from '$lib/components/ui/spinner';
	import BuildingIcon from '@lucide/svelte/icons/building-2';
	import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
	import LockIcon from '@lucide/svelte/icons/lock-keyhole';
	import MailIcon from '@lucide/svelte/icons/mail';
	import { onMount } from 'svelte';
	import {
		companyLocalCurrency,
		isCompanyShareSnapshot,
		type CompanyMetricDisplayCurrency,
		type CompanyShareNarrative,
		type CompanyShareProfile,
		type CompanyShareSnapshot
	} from './company-page-model';
	import { companyPageText } from './company-page-text';
	import CompanyStorySections from './company-story-sections.svelte';
	import TeamActivitySection from './team-activity-section.svelte';

	type PageState = 'loading' | 'locked' | 'unavailable' | 'ready' | 'error';

	let language = $state('en');
	let pageState = $state<PageState>('loading');
	let password = $state('');
	let errorMessage = $state('');
	let snapshot = $state<CompanyShareSnapshot | null>(null);
	let isUnlocking = $state(false);
	let displayCurrency = $state<CompanyMetricDisplayCurrency>('USD');
	const text = $derived(language === 'ko' ? companyPageText.ko : companyPageText.en);
	const profile = $derived(resolveProfile(snapshot, language));
	const narrative = $derived(resolveNarrative(snapshot, language));
	const localCurrency = $derived(snapshot ? companyLocalCurrency(snapshot.metrics, snapshot.records) : undefined);
	const availableLanguages = $derived(snapshot?.languages?.length ? snapshot.languages : ['en']);

	onMount(() => {
		selectLanguage('en');
		loadSession();
	});

	function selectLanguage(selectedLanguage: string): void {
		if (!selectedLanguage) return;
		language = selectedLanguage;
		document.documentElement.lang = selectedLanguage;
	}

	function resolveProfile(currentSnapshot: CompanyShareSnapshot | null, currentLanguage: string): CompanyShareProfile | null {
		if (!currentSnapshot) return null;
		return currentSnapshot.profiles[currentLanguage] ?? currentSnapshot.profiles.en ?? Object.values(currentSnapshot.profiles)[0] ?? null;
	}

	function resolveNarrative(currentSnapshot: CompanyShareSnapshot | null, currentLanguage: string): CompanyShareNarrative {
		return currentSnapshot?.narratives?.[currentLanguage] ?? currentSnapshot?.narratives?.en ?? { highlights: [] };
	}

	function languageLabel(value: string): string {
		try {
			return new Intl.DisplayNames([language], { type: 'language' }).of(value) ?? value;
		} catch {
			return value;
		}
	}

	async function loadSession() {
		try {
			const response = await fetch('/company/api/session', { credentials: 'include' });
			if (!response.ok) {
				pageState = 'error';
				return;
			}
			const session: unknown = await response.json();
			if (!isCompanySession(session)) {
				pageState = 'error';
				return;
			}
			if (!session.available) {
				pageState = 'unavailable';
				return;
			}
			if (!session.authenticated) {
				pageState = 'locked';
				return;
			}
			await loadContent();
		} catch {
			pageState = 'error';
		}
	}

	function isCompanySession(value: unknown): value is { available: boolean; authenticated: boolean } {
		if (!value || typeof value !== 'object') return false;
		const candidate = value as Record<string, unknown>;
		return typeof candidate.available === 'boolean' && typeof candidate.authenticated === 'boolean';
	}

	async function unlockPage(event: SubmitEvent) {
		event.preventDefault();
		if (!password) return;
		isUnlocking = true;
		errorMessage = '';
		try {
			const response = await fetch('/company/api/unlock', {
				method: 'POST', credentials: 'include', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ password })
			});
			if (!response.ok) {
				errorMessage = response.status === 401 ? text.wrongPassword : text.loadError;
				return;
			}
			password = '';
			await loadContent();
		} catch {
			errorMessage = text.loadError;
		} finally {
			isUnlocking = false;
		}
	}

	async function loadContent() {
		const response = await fetch('/company/api/content', { credentials: 'include' });
		if (response.status === 401) {
			pageState = 'locked';
			return;
		}
		if (!response.ok) {
			pageState = 'error';
			return;
		}
		const content: unknown = await response.json();
		if (!isCompanyShareSnapshot(content)) {
			pageState = 'error';
			return;
		}
		snapshot = content;
		displayCurrency = 'USD';
		pageState = 'ready';
	}

	async function lockPage() {
		await fetch('/company/api/logout', { method: 'POST', credentials: 'include' });
		snapshot = null;
		displayCurrency = 'USD';
		pageState = 'locked';
	}

	function selectDisplayCurrency(value: string): void {
		if (value === 'USD' || value === 'local') displayCurrency = value;
	}

	function formatDate(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return value;
		return new Intl.DateTimeFormat(language, { dateStyle: 'medium' }).format(date);
	}

	function companyFacts(currentProfile: CompanyShareProfile) {
		return [
			{ label: text.founded, value: currentProfile.foundedDate },
			{ label: text.teamSize, value: currentProfile.employeeCount ? `${currentProfile.employeeCount} ${text.people}` : '' },
			{ label: text.jurisdiction, value: currentProfile.jurisdiction },
			{ label: text.representative, value: [currentProfile.representativeTitle, currentProfile.representative].filter(Boolean).join(' · ') },
			{ label: text.capital, value: currentProfile.capital },
			{ label: text.fiscalYearEnd, value: currentProfile.fiscalYearEnd }
		].filter((fact) => fact.value);
	}

</script>

<svelte:head>
	<title>{profile?.brandName || profile?.name || 'Company page'}</title>
	<meta name="robots" content="noindex,nofollow" />
</svelte:head>

<main class="bg-background text-foreground min-h-svh">
	<div class="mx-auto flex min-h-svh w-full max-w-6xl flex-col px-5 py-6 sm:px-8 sm:py-8">
		<header class="flex items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="bg-primary text-primary-foreground grid size-9 place-items-center rounded-lg"><BuildingIcon class="size-4" /></div>
				<span class="text-sm font-semibold">{profile?.brandName || profile?.name || 'Company page'}</span>
			</div>
			<div class="flex items-center gap-2">
				{#if availableLanguages.length > 1}
					<Select.Root type="single" value={language} onValueChange={selectLanguage}>
						<Select.Trigger class="h-8 w-auto min-w-28" aria-label={text.changeLanguage}>{languageLabel(language)}</Select.Trigger>
						<Select.Content>
							{#each availableLanguages as availableLanguage}
								<Select.Item value={availableLanguage} label={languageLabel(availableLanguage)}>{languageLabel(availableLanguage)}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{/if}
				{#if pageState === 'ready'}<Button variant="ghost" size="sm" onclick={lockPage}><LockIcon data-icon="inline-start" />{text.lock}</Button>{/if}
			</div>
		</header>

		{#if pageState === 'loading'}
			<div class="grid flex-1 place-items-center"><Spinner class="size-6" /></div>
		{:else if pageState === 'locked'}
			<div class="grid flex-1 place-items-center py-16">
				<Card.Root class="w-full max-w-md">
					<Card.Header>
						<Badge variant="secondary" class="mb-3 w-fit"><LockIcon data-icon="inline-start" />{text.protected}</Badge>
						<Card.Title class="text-xl">{text.unlockTitle}</Card.Title>
						<Card.Description>{text.unlockDescription}</Card.Description>
					</Card.Header>
					<form onsubmit={unlockPage}>
						<Card.Content class="pb-6">
							<Field.Field data-invalid={!!errorMessage}>
								<Field.Label for="company-page-password">{text.password}</Field.Label>
								<Input id="company-page-password" type="password" bind:value={password} placeholder={text.passwordPlaceholder} autocomplete="current-password" aria-invalid={!!errorMessage} autofocus />
								{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
							</Field.Field>
						</Card.Content>
						<Card.Footer><Button type="submit" class="w-full" disabled={!password || isUnlocking}>{#if isUnlocking}<Spinner data-icon="inline-start" />{/if}{text.unlock}</Button></Card.Footer>
					</form>
				</Card.Root>
			</div>
		{:else if pageState === 'unavailable' || pageState === 'error'}
			<div class="grid flex-1 place-items-center py-16 text-center">
				<div class="max-w-md"><h1 class="text-2xl font-semibold">{pageState === 'unavailable' ? text.unavailableTitle : text.loadError}</h1>{#if pageState === 'unavailable'}<p class="text-muted-foreground mt-3">{text.unavailableDescription}</p>{/if}</div>
			</div>
		{:else if snapshot && profile}
			<section class="grid gap-6 py-16 sm:py-24">
				<Badge variant="secondary" class="w-fit"><LockIcon data-icon="inline-start" />{text.protected}</Badge>
				<div class="max-w-4xl">
					<p class="text-muted-foreground text-sm font-medium tracking-wide">{profile.brandName || profile.name}</p>
					<h1 class="mt-4 text-4xl font-semibold tracking-tight text-balance sm:text-6xl">{profile.slogan || profile.name}</h1>
					{#if profile.description}<p class="text-muted-foreground mt-6 max-w-3xl text-lg leading-8 text-pretty">{profile.description}</p>{/if}
				</div>
				<div class="flex flex-wrap gap-3">
					{#if snapshot.contactEmail || profile.email}<Button href={`mailto:${snapshot.contactEmail || profile.email}`}><MailIcon data-icon="inline-start" />{text.contact}</Button>{/if}
					{#if profile.website}<Button variant="outline" href={profile.website} target="_blank" rel="noreferrer"><ExternalLinkIcon data-icon="inline-start" />{text.website}</Button>{/if}
				</div>
			</section>

			<Separator />

			<CompanyStorySections
				metrics={snapshot.metrics}
				primaryMetric={snapshot.primaryMetric}
				metricContexts={snapshot.metricContexts ?? {}}
				records={snapshot.records}
				documents={snapshot.documents ?? []}
				{narrative}
				{displayCurrency}
				{localCurrency}
				{language}
				text={text.story}
				displayCurrencyLabel={text.displayCurrency}
				contactEmail={snapshot.contactEmail || profile.email}
				onDisplayCurrencyChange={selectDisplayCurrency}
			/>

			{#if snapshot.teamActivity}
				<Separator />
				<div class="py-12 sm:py-16">
					<TeamActivitySection activity={snapshot.teamActivity} text={text.teamActivity} {language} />
				</div>
			{/if}

			{#if companyFacts(profile).length > 0}
				<section class="py-14 sm:py-20">
					<h2 class="text-2xl font-semibold">{text.facts}</h2>
					<dl class="mt-7 grid border-y sm:grid-cols-2 lg:grid-cols-3">
						{#each companyFacts(profile) as fact}
							<div class="border-b py-5 sm:px-5 sm:first:pl-0"><dt class="text-muted-foreground text-sm">{fact.label}</dt><dd class="mt-2 font-medium">{fact.value}</dd></div>
						{/each}
					</dl>
				</section>
			{/if}

			<footer class="text-muted-foreground mt-auto flex flex-wrap items-center justify-between gap-3 border-t py-6 text-xs"><span>{profile.name}</span><span>{text.published} {formatDate(snapshot.publishedAt)} · r{snapshot.revision}</span></footer>
		{/if}
	</div>
</main>
