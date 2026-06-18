<script lang="ts">
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import type { MailAccount, MailAccountDraft } from './mail-types';
	import type { mailText } from './text';

	type Props = {
		account: MailAccount;
		accountDraft: MailAccountDraft;
		isAdvancedSettingsOpen: boolean;
		text: (typeof mailText)['ko'];
		syncSMTPAppPassword: (event: Event) => void;
	};

	let {
		account,
		accountDraft = $bindable(),
		isAdvancedSettingsOpen = $bindable(false),
		text,
		syncSMTPAppPassword
	}: Props = $props();

	function toggleAdvancedSettings() {
		isAdvancedSettingsOpen = !isAdvancedSettingsOpen;
	}
</script>

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
						<Input
							id="mail-smtp-password"
							type="password"
							autocomplete="current-password"
							value={accountDraft.smtpPassword}
							placeholder={account.hasSMTPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword}
							oninput={syncSMTPAppPassword}
						/>
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
