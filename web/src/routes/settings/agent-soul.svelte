<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import SoulForm from '$lib/persona/soul-form.svelte';
	import { draftToSoul, soulToDraft, type SoulDraft } from '$lib/persona/soul-draft';
	import { fetchAgentSoul, updateAgentSoul } from './persona-api';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);

	let draft = $state<SoulDraft>(soulToDraft({ schemaVersion: 1 }));
	let errorMessage = $state('');
	let isLoading = $state(true);
	let isSaving = $state(false);

	onMount(async () => {
		try {
			draft = soulToDraft(await fetchAgentSoul());
		} catch {
			errorMessage = text.persona.soulLoadError;
		} finally {
			isLoading = false;
		}
	});

	async function save() {
		isSaving = true;
		errorMessage = '';
		try {
			draft = soulToDraft(await updateAgentSoul(draftToSoul(draft)));
			toast.success(text.persona.saved);
		} catch (error) {
			errorMessage = error instanceof Error && error.message ? error.message : text.persona.soulSaveError;
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.persona.soulTitle}</Card.Title>
		<Card.Description>{text.persona.soulDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-5">
		<SoulForm bind:draft text={text.persona} disabled={isLoading} />
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
