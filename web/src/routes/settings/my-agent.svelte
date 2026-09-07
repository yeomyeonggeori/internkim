<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import * as Alert from '$lib/components/ui/alert';
	import * as Select from '$lib/components/ui/select';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { Input } from '$lib/components/ui/input';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { draftToUser, toneRegisters, toneTraitLimit, toneTraits, userToDraft, type UserDraft } from '$lib/persona/soul-draft';
	import { defaultCallMe, replyLanguageOptions } from '$lib/persona/languages';
	import { fetchMyAgentDocument, updateMyAgentDocument } from './persona-api';
	import { companySettingsText } from './text';
	import { companyTimeZone } from '$lib/company/company-settings';
	import { morningBriefingDefaults } from '$lib/persona/user-schema-defaults';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMember } from '$lib/supabase-session';

	const text = createPageText(companySettingsText);
	const traitLabels: Record<string, string> = text.persona.toneTraits;
	const fieldID = $props.id();

	let draft = $state<UserDraft>(userToDraft({ schemaVersion: 1 }));
	let errorMessage = $state('');
	let isLoading = $state(false);
	let isSaving = $state(false);
	let hasLoaded = $state(false);
	let companyTimeZoneName = $state('');
	let morningBriefing = $state(morningBriefingDefaults());

	const selectableTraits = $derived([...toneTraits, ...draft.traits.filter((trait) => !toneTraits.includes(trait))]);
	const isTraitLimitReached = $derived(draft.traits.length >= toneTraitLimit);
	const languageOptions = $derived(replyLanguageOptions(draft.languageDefault));
	const selectedLanguageLabel = $derived(languageOptions.find((option) => option.value === draft.languageDefault)?.label ?? '');

	onMount(() => {
		void load();
		const resumeLoading = () => {
			if (document.visibilityState === 'visible') void load();
		};
		window.addEventListener('online', resumeLoading);
		window.addEventListener('focus', resumeLoading);
		document.addEventListener('visibilitychange', resumeLoading);
		return () => {
			window.removeEventListener('online', resumeLoading);
			window.removeEventListener('focus', resumeLoading);
			document.removeEventListener('visibilitychange', resumeLoading);
		};
	});

	async function load() {
		if (isLoading || hasLoaded) return;
		isLoading = true;
		errorMessage = '';
		try {
			draft = userToDraft(await fetchMyAgentDocument());
			morningBriefing = { ...morningBriefingDefaults(), ...draft.morningBriefing };
			await Promise.all([fillDefaultsFromMembership(), loadCompanyTimeZone()]);
			hasLoaded = true;
		} catch {
			errorMessage = text.persona.userLoadError;
		} finally {
			isLoading = false;
		}
	}

	async function fillDefaultsFromMembership() {
		if (!isSupabaseConfigured()) return;
		if (draft.callMe && draft.languageDefault) return;
		const member = await supabaseMember();
		if (!draft.languageDefault && member.companyLocale) draft.languageDefault = member.companyLocale;
		if (!draft.callMe) draft.callMe = defaultCallMe(member.name, draft.languageDefault);
	}

	async function loadCompanyTimeZone() {
		try {
			companyTimeZoneName = await companyTimeZone();
		} catch {
			companyTimeZoneName = '';
		}
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (!hasLoaded || isSaving) return;
		isSaving = true;
		errorMessage = '';
		try {
			draft = userToDraft(await updateMyAgentDocument(draftToUser({ ...draft, morningBriefing })));
			morningBriefing = { ...morningBriefingDefaults(), ...draft.morningBriefing };
			toast.success(text.persona.saved);
		} catch (error) {
			errorMessage = error instanceof Error && error.message ? error.message : text.persona.userSaveError;
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.persona.userTitle}</Card.Title>
		<Card.Description>{text.persona.userDescription}</Card.Description>
	</Card.Header>
	<form aria-label={text.persona.userTitle} onsubmit={save} class="grid gap-6">
		<Card.Content class="grid gap-5">
			{#if !hasLoaded}
				{#if isLoading}
					<p role="status">{text.persona.loading}</p>
				{:else if errorMessage}
					<Alert.Root variant="destructive">
						<Alert.Title>{errorMessage}</Alert.Title>
					</Alert.Root>
					<Button type="button" variant="outline" onclick={load} class="w-fit">{text.persona.retryLoad}</Button>
				{/if}
			{:else}
				<Field.Group class="grid gap-5 md:grid-cols-2">
					<Field.Field>
						<Field.Label for="{fieldID}-call-me">{text.persona.callMeLabel}</Field.Label>
						<Input id="{fieldID}-call-me" bind:value={draft.callMe} placeholder={text.persona.callMePlaceholder} disabled={isLoading || isSaving} />
					</Field.Field>
					<Field.Field>
						<Field.Label for="{fieldID}-language">{text.persona.userLanguageLabel}</Field.Label>
						<Select.Root type="single" bind:value={draft.languageDefault} disabled={isLoading || isSaving}>
							<Select.Trigger id="{fieldID}-language" class="w-full">
								{selectedLanguageLabel}
							</Select.Trigger>
							<Select.Content>
								<Select.Group>
									{#each languageOptions as option (option.value)}
										<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
									{/each}
								</Select.Group>
							</Select.Content>
						</Select.Root>
					</Field.Field>
				</Field.Group>
				<Field.Field>
					<Field.Label for="{fieldID}-about">{text.persona.aboutLabel}</Field.Label>
					<Textarea id="{fieldID}-about" bind:value={draft.about} placeholder={text.persona.aboutPlaceholder} disabled={isLoading || isSaving} class="min-h-20" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="{fieldID}-preferences">{text.persona.preferencesLabel}</Field.Label>
					<Textarea id="{fieldID}-preferences" bind:value={draft.preferencesText} placeholder={text.persona.linesPlaceholder} disabled={isLoading || isSaving} class="min-h-28" />
				</Field.Field>
				<Field.Group class="grid gap-5 md:grid-cols-2">
					<Field.Field>
						<Field.Label for="{fieldID}-register">{text.persona.toneRegisterLabel}</Field.Label>
						<Select.Root type="single" bind:value={draft.register} disabled={isLoading || isSaving}>
							<Select.Trigger id="{fieldID}-register" class="w-full">
								{text.persona.toneRegisters[draft.register]}
							</Select.Trigger>
							<Select.Content>
								<Select.Group>
									{#each toneRegisters as register (register)}
										<Select.Item value={register} label={text.persona.toneRegisters[register]}>{text.persona.toneRegisters[register]}</Select.Item>
									{/each}
								</Select.Group>
							</Select.Content>
						</Select.Root>
					</Field.Field>
					<Field.Field>
						<Field.Label>{text.persona.traitsLabel}</Field.Label>
						<ToggleGroup.Root type="multiple" bind:value={draft.traits} variant="outline" size="sm" spacing={2} disabled={isLoading || isSaving} aria-label={text.persona.traitsLabel} class="flex-wrap justify-start">
							{#each selectableTraits as trait (trait)}
								<ToggleGroup.Item value={trait} disabled={isLoading || isSaving || (isTraitLimitReached && !draft.traits.includes(trait))}>
									{traitLabels[trait] || trait}
								</ToggleGroup.Item>
							{/each}
						</ToggleGroup.Root>
					</Field.Field>
				</Field.Group>
				<Field.Set>
					<Field.Legend>{text.persona.morningBriefingTitle}</Field.Legend>
					<Field.Description>
						{text.persona.morningBriefingDescription}
					</Field.Description>
					<Field.Group>
						<Field.Field orientation="horizontal">
							<Field.Label for="{fieldID}-morning-briefing-enabled">{text.persona.morningBriefingEnabled}</Field.Label>
							<Switch id="{fieldID}-morning-briefing-enabled" bind:checked={morningBriefing.enabled} disabled={isLoading || isSaving} />
						</Field.Field>
						<Field.Field>
							<Field.Label for="{fieldID}-morning-briefing-time">{text.persona.morningBriefingTime}</Field.Label>
							<Input id="{fieldID}-morning-briefing-time" type="time" bind:value={morningBriefing.time} required disabled={isLoading || isSaving} />
							<Field.Description>
								{text.persona.morningBriefingTimeZone}{#if companyTimeZoneName}: {companyTimeZoneName}{/if}
							</Field.Description>
						</Field.Field>
					</Field.Group>
				</Field.Set>
				{#if errorMessage}
					<Field.Error>{errorMessage}</Field.Error>
				{/if}
			{/if}
		</Card.Content>
		{#if hasLoaded}
			<Card.Footer class="justify-end">
				<Button type="submit" disabled={!hasLoaded || isLoading || isSaving}>
					{#if isSaving}
						<LoaderIcon data-icon="inline-start" class="animate-spin" />
					{/if}
					{text.persona.save}
				</Button>
			</Card.Footer>
		{/if}
	</form>
</Card.Root>
