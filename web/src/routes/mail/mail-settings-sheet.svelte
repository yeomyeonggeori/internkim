<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
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
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>{text.settingsSheet.title}</Sheet.Title>
			<Sheet.Description>{text.settingsSheet.description}</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-5 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); saveAccount(); }}>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-email">{text.fields.emailAddress}</Label>
					<Input id="mail-email" bind:value={accountDraft.email} placeholder="you@example.com" />
				</div>
				<div class="space-y-2">
					<Label for="mail-display-name">{text.fields.displayName}</Label>
					<Input id="mail-display-name" bind:value={accountDraft.displayName} placeholder={text.settingsSheet.displayNamePlaceholder} />
				</div>
				<div class="space-y-2">
					<Label for="mail-default-mailbox">{text.fields.defaultMailbox}</Label>
					<Input id="mail-default-mailbox" bind:value={accountDraft.defaultMailbox} />
				</div>
			</div>

			<Separator />

			<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_96px_120px]">
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
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-imap-user">IMAP {text.fields.username}</Label>
					<Input id="mail-imap-user" bind:value={accountDraft.imapUsername} />
				</div>
				<div class="space-y-2">
					<Label for="mail-imap-password">IMAP {text.fields.password}</Label>
					<Input id="mail-imap-password" type="password" bind:value={accountDraft.imapPassword} placeholder={account.hasIMAPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword} />
				</div>
			</div>

			<Separator />

			<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_96px_120px]">
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
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-smtp-user">SMTP {text.fields.username}</Label>
					<Input id="mail-smtp-user" bind:value={accountDraft.smtpUsername} />
				</div>
				<div class="space-y-2">
					<Label for="mail-smtp-password">SMTP {text.fields.password}</Label>
					<Input id="mail-smtp-password" type="password" bind:value={accountDraft.smtpPassword} placeholder={account.hasSMTPPassword ? text.settingsSheet.savedPassword : text.settingsSheet.appPassword} />
				</div>
				<div class="space-y-2">
					<Label for="mail-sent-mailbox">{text.fields.sentMailbox}</Label>
					<Input id="mail-sent-mailbox" bind:value={accountDraft.sentMailbox} />
				</div>
			</div>

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
