<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
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

<div class="rounded-lg border p-4">
	<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
		<div>
			<h3 class="text-sm font-semibold">{text.bot.title}</h3>
			<p class="text-muted-foreground mt-1 text-sm">
				{text.bot.description}
			</p>
		</div>
		<Badge variant="outline">{botProfile.username}</Badge>
	</div>
	<div class="grid gap-3 md:grid-cols-2">
		<Input bind:value={botProfile.displayName} placeholder={text.bot.displayNamePlaceholder} disabled={isLoadingBotProfile} />
		<Input bind:value={botProfile.englishDisplayName} placeholder={text.bot.englishDisplayNamePlaceholder} disabled={isLoadingBotProfile} />
		<Input
			class="md:col-span-2"
			bind:value={botProfile.publicDescription}
			placeholder={text.bot.publicDescriptionPlaceholder}
			disabled={isLoadingBotProfile}
		/>
		<Textarea
			bind:value={botProfileAliasesText}
			placeholder={text.bot.aliasesPlaceholder}
			disabled={isLoadingBotProfile}
			class="min-h-24"
		/>
		<Textarea
			bind:value={botProfile.identityExtension}
			placeholder={text.bot.identityExtensionPlaceholder}
			disabled={isLoadingBotProfile}
			class="min-h-24"
		/>
	</div>
	<div class="mt-3 flex flex-wrap items-center justify-between gap-3">
		<p class="text-muted-foreground text-xs">
			{text.bot.identityNotice}
		</p>
		<Button disabled={!isDeviceReachable || isSavingBotProfile || !botProfile.displayName.trim()} onclick={saveBotProfile}>
			{#if isSavingBotProfile}
				<LoaderIcon class="size-4 animate-spin" />
			{/if}
			{text.bot.save}
		</Button>
	</div>
	{#if botProfileErrorMessage}
		<p class="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
			{botProfileErrorMessage}
		</p>
	{/if}
</div>
