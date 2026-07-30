<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { apiErrorMessage, fetchBotProfile, updateBotProfile } from './admin-api';
	import type { AdminPageText, BotProfile } from './admin-types';

	type BotSectionProps = {
		adminBaseURL: string;
		isDeviceReachable: boolean;
		text: AdminPageText;
	};

	let { adminBaseURL, isDeviceReachable, text }: BotSectionProps = $props();

	let loadedAdminBaseURL = $state('');
	let botProfile = $state<BotProfile>(defaultBotProfile());
	let botProfileAliasesText = $state('인턴킴\nintern kim');
	let botProfileErrorMessage = $state('');
	let isLoadingBotProfile = $state(false);
	let isSavingBotProfile = $state(false);

	$effect(() => {
		if (!adminBaseURL || loadedAdminBaseURL === adminBaseURL) return;
		loadedAdminBaseURL = adminBaseURL;
		loadBotProfile();
	});

	function defaultBotProfile(): BotProfile {
		return {
			username: 'internkim',
			displayName: '김인턴',
			englishDisplayName: 'Intern Kim',
			aliases: ['인턴킴', 'intern kim'],
			publicDescription: '',
			identityExtension: ''
		};
	}

	function applyBotProfile(profile: BotProfile) {
		botProfile = {
			username: profile.username || 'internkim',
			displayName: profile.displayName || '김인턴',
			englishDisplayName: profile.englishDisplayName || 'Intern Kim',
			aliases: profile.aliases ?? [],
			publicDescription: profile.publicDescription || '',
			identityExtension: profile.identityExtension || ''
		};
		botProfileAliasesText = (botProfile.aliases ?? []).join('\n');
	}

	async function loadBotProfile() {
		if (!adminBaseURL) return;

		isLoadingBotProfile = true;
		botProfileErrorMessage = '';
		try {
			applyBotProfile(await fetchBotProfile(adminBaseURL, text.messages.botProfileLoadError));
		} catch {
			botProfileErrorMessage = text.messages.botProfileLoadError;
		} finally {
			isLoadingBotProfile = false;
		}
	}

	async function saveBotProfile() {
		if (!adminBaseURL) return;

		isSavingBotProfile = true;
		botProfileErrorMessage = '';
		try {
			const profile = {
				...botProfile,
				username: 'internkim',
				aliases: botProfileAliasesText
					.split('\n')
					.map((alias) => alias.trim())
					.filter(Boolean)
			};
			applyBotProfile(await updateBotProfile(adminBaseURL, profile, text.messages.botProfileSaveError));
		} catch (error) {
			botProfileErrorMessage = apiErrorMessage(error, text.messages.botProfileSaveError);
		} finally {
			isSavingBotProfile = false;
		}
	}
</script>

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.bot.title}</Card.Title>
		<Card.Description>{text.bot.description}</Card.Description>
		<Card.Action>
			<Badge variant="outline" class="font-mono">{botProfile.username}</Badge>
		</Card.Action>
	</Card.Header>
	<Card.Content>
		<Field.Group>
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="bot-display-name">{text.bot.displayNamePlaceholder}</Field.Label>
					<Input id="bot-display-name" bind:value={botProfile.displayName} disabled={isLoadingBotProfile} />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-english-display-name">{text.bot.englishDisplayNamePlaceholder}</Field.Label>
					<Input id="bot-english-display-name" bind:value={botProfile.englishDisplayName} disabled={isLoadingBotProfile} />
				</Field.Field>
			</div>
			<Field.Field>
				<Field.Label for="bot-public-description">{text.bot.publicDescriptionPlaceholder}</Field.Label>
				<Input id="bot-public-description" bind:value={botProfile.publicDescription} disabled={isLoadingBotProfile} />
			</Field.Field>
			<div class="grid gap-5 md:grid-cols-2">
				<Field.Field>
					<Field.Label for="bot-aliases">{text.bot.aliasesLabel}</Field.Label>
					<Textarea id="bot-aliases" bind:value={botProfileAliasesText} placeholder={text.bot.aliasesPlaceholder} disabled={isLoadingBotProfile} class="min-h-24" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="bot-identity-extension">{text.bot.identityExtensionPlaceholder}</Field.Label>
					<Textarea id="bot-identity-extension" bind:value={botProfile.identityExtension} disabled={isLoadingBotProfile} class="min-h-24" />
					<Field.Description>{text.bot.identityNotice}</Field.Description>
				</Field.Field>
			</div>
			{#if botProfileErrorMessage}
				<Field.Error>{botProfileErrorMessage}</Field.Error>
			{/if}
		</Field.Group>
	</Card.Content>
	<Card.Footer class="justify-end">
		<Button disabled={!isDeviceReachable || isSavingBotProfile || !botProfile.displayName.trim()} onclick={saveBotProfile}>
			{#if isSavingBotProfile}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.bot.save}
		</Button>
	</Card.Footer>
</Card.Root>
