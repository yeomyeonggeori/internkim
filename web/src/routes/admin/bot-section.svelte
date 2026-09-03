<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import SoulForm from '$lib/persona/soul-form.svelte';
	import { draftToSoul, soulToDraft, type SoulDraft } from '$lib/persona/soul-draft';
	import { apiErrorMessage, fetchSoul, updateSoul } from './admin-api';
	import type { AdminPageText } from './admin-types';

	type BotSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: BotSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let draft = $state<SoulDraft>(soulToDraft({ schemaVersion: 1 }));
	let errorMessage = $state('');
	let isLoading = $state(false);
	let isSaving = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadSoul();
	});

	async function loadSoul() {
		if (!adminBaseURL) return;
		isLoading = true;
		errorMessage = '';
		try {
			draft = soulToDraft(await fetchSoul(adminBaseURL, text.messages.botProfileLoadError));
		} catch {
			errorMessage = text.messages.botProfileLoadError;
		} finally {
			isLoading = false;
		}
	}

	async function saveSoul() {
		if (!adminBaseURL) return;
		isSaving = true;
		errorMessage = '';
		try {
			draft = soulToDraft(await updateSoul(adminBaseURL, draftToSoul(draft), text.messages.botProfileSaveError));
		} catch (error) {
			errorMessage = apiErrorMessage(error, text.messages.botProfileSaveError);
		} finally {
			isSaving = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.bot.title}</Card.Title>
		<Card.Description>{text.bot.description}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-5">
		<SoulForm bind:draft text={text.bot} disabled={isLoading} />
		{#if errorMessage}
			<Field.Error>{errorMessage}</Field.Error>
		{/if}
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={!isDeviceReachable || isSaving || isLoading} onclick={saveSoul}>
			{#if isSaving}
				<LoaderIcon class="animate-spin" />
			{/if}
			{text.bot.save}
		</Button>
	</Card.Footer>
</Card.Root>
