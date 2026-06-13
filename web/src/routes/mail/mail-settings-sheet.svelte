<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import {
		currentMailPresetFromDraft,
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
		const appPassword = (event.currentTarget as HTMLInputElement).value;
		accountDraft.imapPassword = appPassword;
		accountDraft.smtpPassword = appPassword;
	}

	function toggleAdvancedSettings() {
		isAdvancedSettingsOpen = !isAdvancedSettingsOpen;
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="overflow-y-auto data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>{text.settingsSheet.title}</Sheet.Title>
			<Sheet.Description>{text.settingsSheet.description}</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-4 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); saveAccount(); }}>
			<div class="grid gap-4">
				<div class="space-y-2 sm:col-span-2">
					<Label for="mail-email">{text.fields.emailAddress}</Label>
					<div class="grid grid-cols-[minmax(0,1fr)_auto_minmax(120px,0.8fr)] items-center gap-2">
						<Input
							id="mail-email"
							bind:value={emailLocalPart}
							placeholder="example"
							oninput={() => syncAccountEmailFromParts(false)}
						/>
						<span class="text-sm font-medium text-muted-foreground">@</span>
						<select
							id="mail-email-provider"
							aria-label={text.fields.emailDomain}
							class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm"
							bind:value={emailProviderID}
							onchange={handleEmailProviderChange}
						>
							<option value="gmail">{text.providers.gmail}</option>
							<option value="naver">{text.providers.naver}</option>
							<option value="custom">{text.providers.custom}</option>
						</select>
					</div>
					{#if emailProviderID === 'custom'}
						<div class="space-y-2">
							<Label for="mail-custom-domain">{text.fields.customDomain}</Label>
							<Input
								id="mail-custom-domain"
								bind:value={customEmailDomain}
								placeholder="example.com"
								oninput={() => syncAccountEmailFromParts(false)}
							/>
						</div>
					{/if}
					<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.emailAddress}</p>
				</div>
				<div class="grid w-full gap-3 sm:col-span-2">
					<div class="w-full space-y-2">
						<Label for="mail-app-password">{text.fields.appPassword}</Label>
						<Input
							id="mail-app-password"
							type="password"
							value={accountDraft.imapPassword}
							placeholder={account.hasIMAPPassword || account.hasSMTPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword}
							oninput={syncCommonAppPassword}
						/>
						<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.appPassword}</p>
					</div>
					<div class="w-full space-y-2">
						<Label for="mail-display-name">{text.fields.displayName}</Label>
						<Input id="mail-display-name" bind:value={accountDraft.displayName} placeholder={text.settingsSheet.displayNamePlaceholder} />
						<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.displayName}</p>
					</div>
				</div>
			</div>

			<Separator />

			<div class="rounded-lg border bg-muted/20">
				<button
					type="button"
					class="flex w-full items-center justify-between gap-3 px-3 py-3 text-left"
					aria-expanded={isAdvancedSettingsOpen}
					aria-controls="mail-advanced-settings"
					onclick={toggleAdvancedSettings}
				>
					<span>
						<span class="block text-sm font-medium">{text.settingsSheet.advancedSettings}</span>
						<span class="block text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.providerPreset}</span>
					</span>
					{#if isAdvancedSettingsOpen}
						<ChevronUpIcon class="h-4 w-4 shrink-0 text-muted-foreground" />
					{:else}
						<ChevronDownIcon class="h-4 w-4 shrink-0 text-muted-foreground" />
					{/if}
				</button>
				{#if isAdvancedSettingsOpen}
					<div id="mail-advanced-settings" class="grid gap-5 border-t bg-background px-3 py-4">
						<div class="grid gap-3">
							<div class="space-y-2">
								<Label for="mail-imap-host">IMAP {text.fields.host}</Label>
								<Input id="mail-imap-host" bind:value={accountDraft.imapHost} placeholder="imap.gmail.com" />
							</div>
							<div class="space-y-2">
								<Label for="mail-imap-port">{text.fields.port}</Label>
								<Input id="mail-imap-port" type="number" bind:value={accountDraft.imapPort} />
							</div>
							<div class="space-y-2">
								<Label for="mail-imap-security">{text.fields.security}</Label>
								<select id="mail-imap-security" class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm" bind:value={accountDraft.imapSecurity}>
									<option value="tls">SSL/TLS</option>
									<option value="starttls">STARTTLS</option>
									<option value="none">{text.settingsSheet.securityNone}</option>
								</select>
							</div>
						</div>
						<div class="grid gap-3">
							<div class="space-y-2">
								<Label for="mail-imap-user">IMAP {text.fields.loginAccount}</Label>
								<Input id="mail-imap-user" bind:value={accountDraft.imapUsername} />
								<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.loginAccount}</p>
							</div>
							<div class="space-y-2">
								<Label for="mail-default-mailbox">{text.fields.defaultMailbox}</Label>
								<Input id="mail-default-mailbox" bind:value={accountDraft.defaultMailbox} />
							</div>
						</div>
						<Separator />
						<div class="grid gap-3">
							<div class="space-y-2">
								<Label for="mail-smtp-host">SMTP {text.fields.host}</Label>
								<Input id="mail-smtp-host" bind:value={accountDraft.smtpHost} placeholder="smtp.gmail.com" />
							</div>
							<div class="space-y-2">
								<Label for="mail-smtp-port">{text.fields.port}</Label>
								<Input id="mail-smtp-port" type="number" bind:value={accountDraft.smtpPort} />
							</div>
							<div class="space-y-2">
								<Label for="mail-smtp-security">{text.fields.security}</Label>
								<select id="mail-smtp-security" class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm" bind:value={accountDraft.smtpSecurity}>
									<option value="tls">SSL/TLS</option>
									<option value="starttls">STARTTLS/TLS</option>
									<option value="none">{text.settingsSheet.securityNone}</option>
								</select>
							</div>
						</div>
						<div class="grid gap-3">
							<div class="space-y-2">
								<Label for="mail-smtp-user">SMTP {text.fields.loginAccount}</Label>
								<Input id="mail-smtp-user" bind:value={accountDraft.smtpUsername} />
								<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.loginAccount}</p>
							</div>
							<div class="space-y-2">
								<Label for="mail-smtp-password">SMTP {text.fields.appPassword}</Label>
								<Input id="mail-smtp-password" type="password" bind:value={accountDraft.smtpPassword} placeholder={account.hasSMTPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword} />
							</div>
							<div class="space-y-2">
								<Label for="mail-sent-mailbox">{text.fields.sentMailbox}</Label>
								<Input id="mail-sent-mailbox" bind:value={accountDraft.sentMailbox} />
								<p class="text-xs leading-5 text-muted-foreground">{text.fieldDescriptions.sentMailbox}</p>
							</div>
						</div>
					</div>
				{/if}
			</div>

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
