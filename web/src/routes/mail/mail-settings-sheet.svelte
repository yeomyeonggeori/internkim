<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import MailSettingsAccountFields from './mail-settings-account-fields.svelte';
	import MailSettingsAdvancedFields from './mail-settings-advanced-fields.svelte';
	import {
		currentMailPresetFromDraft,
		mailAppPasswordInputValue,
		mailAddressSettingsUpdate,
		mailProviderSettingsUpdate,
		mailSettingsEmailStateFromEmail,
		selectedMailSettingsDomain
	} from './mail-settings-state';
	import {
		DEFAULT_MAIL_PROVIDER_ID,
		type MailProviderID
	} from './mail-provider-presets';
	import type { MailAccount, MailAccountDraft } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		open: boolean;
		account: MailAccount;
		accountDraft: MailAccountDraft;
		settingsMessage: string;
		isSavingAccount: boolean;
		isTestingAccount: boolean;
		text: (typeof mailText)['ko'];
		saveAccount: () => void | Promise<void>;
		testAccount: () => void | Promise<void>;
	};

	let {
		open = $bindable(false),
		account,
		accountDraft = $bindable(),
		settingsMessage,
		isSavingAccount,
		isTestingAccount,
		text,
		saveAccount,
		testAccount
	}: Props = $props();

	let emailLocalPart = $state('');
	let emailProviderID = $state<MailProviderID>(DEFAULT_MAIL_PROVIDER_ID);
	let customEmailDomain = $state('');
	let lastAccountDraftEmail = $state<string | null>(null);
	let isAdvancedSettingsOpen = $state(false);

	$effect(() => {
		const currentEmail = accountDraft.email;
		if (currentEmail === lastAccountDraftEmail) return;
		const emailState = mailSettingsEmailStateFromEmail(currentEmail);
		emailLocalPart = emailState.emailLocalPart;
		emailProviderID = emailState.emailProviderID;
		customEmailDomain = emailState.customEmailDomain;
		isAdvancedSettingsOpen = emailState.isAdvancedSettingsOpen;
		lastAccountDraftEmail = currentEmail;
	});

	function handleEmailProviderChange(event: Event) {
		const previousDomain = currentMailPresetFromDraft(accountDraft)?.domain || selectedMailSettingsDomain(emailProviderID, customEmailDomain);
		emailProviderID = (event.currentTarget as HTMLSelectElement).value as MailProviderID;
		if (emailProviderID === 'custom' && !customEmailDomain) {
			customEmailDomain = previousDomain;
		}
		isAdvancedSettingsOpen = emailProviderID === 'custom';
		applyMailProviderPreset(emailProviderID);
	}

	function applyMailProviderPreset(providerID: MailProviderID) {
		Object.assign(accountDraft, mailProviderSettingsUpdate(accountDraft, providerID));
		syncAccountEmailFromParts(true);
	}

	function syncAccountEmailFromParts(forceUsernameSync: boolean) {
		Object.assign(accountDraft, mailAddressSettingsUpdate(accountDraft, emailLocalPart, emailProviderID, customEmailDomain, forceUsernameSync));
		lastAccountDraftEmail = accountDraft.email;
	}

	function syncCommonAppPassword(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const appPassword = mailAppPasswordInputValue(input.value, emailProviderID);
		input.value = appPassword;
		accountDraft.imapPassword = appPassword;
		accountDraft.smtpPassword = appPassword;
	}

	function syncSMTPAppPassword(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const appPassword = mailAppPasswordInputValue(input.value, emailProviderID);
		input.value = appPassword;
		accountDraft.smtpPassword = appPassword;
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>{text.settingsSheet.title}</Sheet.Title>
			<Sheet.Description>{text.settingsSheet.description}</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-4 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); saveAccount(); }}>
			<MailSettingsAccountFields
				account={account}
				bind:accountDraft
				bind:emailLocalPart
				bind:emailProviderID
				bind:customEmailDomain
				text={text}
				handleEmailProviderChange={handleEmailProviderChange}
				syncAccountEmailFromParts={syncAccountEmailFromParts}
				syncCommonAppPassword={syncCommonAppPassword}
			/>

			<Separator />

			<MailSettingsAdvancedFields
				account={account}
				bind:accountDraft
				bind:isAdvancedSettingsOpen
				text={text}
				syncSMTPAppPassword={syncSMTPAppPassword}
			/>

			{#if emailProviderID !== 'custom'}
				<div class="rounded-md bg-muted/40 px-3 py-2 text-xs leading-5 text-muted-foreground">
					{text.settingsSheet.autoConfigured}
				</div>
			{/if}

			{#if settingsMessage}
				<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{settingsMessage}</p>
			{/if}

			<Sheet.Footer class="gap-2 sm:justify-between">
				<Button type="button" variant="outline" onclick={testAccount} disabled={isTestingAccount || isSavingAccount}>
					{isTestingAccount ? text.settingsSheet.testing : text.settingsSheet.testConnection}
				</Button>
				<Button type="submit" disabled={isSavingAccount || isTestingAccount}>
					{isSavingAccount ? text.settingsSheet.saving : text.settingsSheet.save}
				</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
