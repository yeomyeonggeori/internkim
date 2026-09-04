<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import * as Select from '$lib/components/ui/select';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { draftToUser, toneRegisters, toneTraitLimit, toneTraits, userToDraft, type UserDraft } from '$lib/persona/soul-draft';
	import { replyLanguageOptions } from '$lib/persona/languages';
	import { fetchMyAgentDocument, updateMyAgentDocument } from './persona-api';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { isSupabaseConfigured, supabaseMember } from '$lib/supabase-session';

	const text = createPageText(companySettingsText);
	const traitLabels: Record<string, string> = text.persona.toneTraits;
	const fieldID = $props.id();

	let draft = $state<UserDraft>(userToDraft({ schemaVersion: 1 }));
	let errorMessage = $state('');
	let isLoading = $state(true);
	let isSaving = $state(false);

	const selectableTraits = $derived([...toneTraits, ...draft.traits.filter((trait) => !toneTraits.includes(trait))]);
	const isTraitLimitReached = $derived(draft.traits.length >= toneTraitLimit);
	const languageOptions = $derived(replyLanguageOptions(draft.languageDefault));
	const selectedLanguageLabel = $derived(languageOptions.find((option) => option.value === draft.languageDefault)?.label ?? '');

	onMount(async () => {
		try {
			draft = userToDraft(await fetchMyAgentDocument());
			await fillDefaultsFromMembership();
		} catch {
			errorMessage = text.persona.userLoadError;
		} finally {
			isLoading = false;
		}
	});

	async function fillDefaultsFromMembership() {
		if (!isSupabaseConfigured()) return;
		if (draft.callMe && draft.languageDefault) return;
		const member = await supabaseMember();
		if (!draft.callMe && member.name) draft.callMe = member.name;
		if (!draft.languageDefault && member.companyLocale) draft.languageDefault = member.companyLocale;
	}

	async function save() {
		isSaving = true;
		errorMessage = '';
		try {
			draft = userToDraft(await updateMyAgentDocument(draftToUser(draft)));
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
	<Card.Content class="grid gap-5">
		<div class="grid gap-5 md:grid-cols-2">
			<Field.Field>
				<Field.Label for="{fieldID}-call-me">{text.persona.callMeLabel}</Field.Label>
				<Input id="{fieldID}-call-me" bind:value={draft.callMe} placeholder={text.persona.callMePlaceholder} disabled={isLoading} />
			</Field.Field>
			<Field.Field>
				<Field.Label for="{fieldID}-language">{text.persona.userLanguageLabel}</Field.Label>
				<Select.Root type="single" bind:value={draft.languageDefault} disabled={isLoading}>
					<Select.Trigger id="{fieldID}-language" class="w-full">
						{selectedLanguageLabel}
					</Select.Trigger>
					<Select.Content>
						{#each languageOptions as option (option.value)}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</Field.Field>
		</div>
		<Field.Field>
			<Field.Label for="{fieldID}-about">{text.persona.aboutLabel}</Field.Label>
			<Textarea id="{fieldID}-about" bind:value={draft.about} placeholder={text.persona.aboutPlaceholder} disabled={isLoading} class="min-h-20" />
		</Field.Field>
		<Field.Field>
			<Field.Label for="{fieldID}-preferences">{text.persona.preferencesLabel}</Field.Label>
			<Textarea id="{fieldID}-preferences" bind:value={draft.preferencesText} placeholder={text.persona.linesPlaceholder} disabled={isLoading} class="min-h-28" />
		</Field.Field>
		<div class="grid gap-5 md:grid-cols-2">
			<Field.Field>
				<Field.Label for="{fieldID}-register">{text.persona.toneRegisterLabel}</Field.Label>
				<Select.Root type="single" bind:value={draft.register} disabled={isLoading}>
					<Select.Trigger id="{fieldID}-register" class="w-full">
						{text.persona.toneRegisters[draft.register]}
					</Select.Trigger>
					<Select.Content>
						{#each toneRegisters as register (register)}
							<Select.Item value={register} label={text.persona.toneRegisters[register]}>{text.persona.toneRegisters[register]}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</Field.Field>
			<Field.Field>
				<Field.Label>{text.persona.traitsLabel}</Field.Label>
				<ToggleGroup.Root type="multiple" bind:value={draft.traits} variant="outline" size="sm" disabled={isLoading} class="flex-wrap justify-start">
					{#each selectableTraits as trait (trait)}
						<ToggleGroup.Item value={trait} disabled={isLoading || (isTraitLimitReached && !draft.traits.includes(trait))}>
							{traitLabels[trait] || trait}
						</ToggleGroup.Item>
					{/each}
				</ToggleGroup.Root>
			</Field.Field>
		</div>
		{#if errorMessage}
			<Field.Error>{errorMessage}</Field.Error>
		{/if}
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={isLoading || isSaving} onclick={save}>
			{#if isSaving}
				<LoaderIcon class="animate-spin" />
			{/if}
			{text.persona.save}
		</Button>
	</Card.Footer>
</Card.Root>
