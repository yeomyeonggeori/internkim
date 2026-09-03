<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { draftToUser, toneRegisters, userToDraft, type UserDraft } from '$lib/persona/soul-draft';
	import { fetchMyAgentDocument, updateMyAgentDocument } from './persona-api';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let draft = $state<UserDraft>(userToDraft({ schemaVersion: 1 }));
	let errorMessage = $state('');
	let isLoading = $state(true);
	let isSaving = $state(false);

	onMount(async () => {
		try {
			draft = userToDraft(await fetchMyAgentDocument());
		} catch {
			errorMessage = text.persona.userLoadError;
		} finally {
			isLoading = false;
		}
	});

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
				<Input id="{fieldID}-language" bind:value={draft.languageDefault} placeholder="ko" disabled={isLoading} />
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
				<select id="{fieldID}-register" bind:value={draft.register} disabled={isLoading} class="border-input bg-background h-9 rounded-md border px-3 text-sm">
					<option value="">{text.persona.toneRegisterUnset}</option>
					{#each toneRegisters as register (register)}
						<option value={register}>{text.persona.toneRegisters[register]}</option>
					{/each}
				</select>
			</Field.Field>
			<Field.Field>
				<Field.Label for="{fieldID}-traits">{text.persona.traitsLabel}</Field.Label>
				<Textarea id="{fieldID}-traits" bind:value={draft.traitsText} placeholder={text.persona.traitsPlaceholder} disabled={isLoading} class="min-h-20" />
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
