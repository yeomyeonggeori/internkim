<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import ArchiveIcon from '@lucide/svelte/icons/archive';
	import FileTextIcon from '@lucide/svelte/icons/file-text';
	import InboxIcon from '@lucide/svelte/icons/inbox';
	import MailIcon from '@lucide/svelte/icons/mail';
	import MailOpenIcon from '@lucide/svelte/icons/mail-open';
	import PanelLeftIcon from '@lucide/svelte/icons/panel-left';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import SendIcon from '@lucide/svelte/icons/send';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import { onMount } from 'svelte';

	type MailAccount = {
		email: string;
		fromAddress: string;
		displayName: string;
		imapHost: string;
		imapPort: number;
		imapSecurity: string;
		imapUsername: string;
		smtpHost: string;
		smtpPort: number;
		smtpSecurity: string;
		smtpUsername: string;
		defaultMailbox: string;
		sentMailbox: string;
		isConfigured: boolean;
		hasIMAPPassword: boolean;
		hasSMTPPassword: boolean;
	};

	type MailAccountDraft = MailAccount & {
		imapPassword: string;
		smtpPassword: string;
	};

	type Mailbox = {
		name: string;
		displayName: string;
		unseen: number;
		total: number;
	};

	type MailMessage = {
		uid: number;
		mailbox: string;
		subject: string;
		from: string;
		to?: string;
		cc?: string;
		date: string;
		preview: string;
		body?: string;
		isRead: boolean;
	};

	type ComposeDraft = {
		to: string;
		cc: string;
		bcc: string;
		subject: string;
		body: string;
	};

	const emptyAccount: MailAccount = {
		email: '',
		fromAddress: '',
		displayName: '',
		imapHost: '',
		imapPort: 993,
		imapSecurity: 'tls',
		imapUsername: '',
		smtpHost: '',
		smtpPort: 587,
		smtpSecurity: 'starttls',
		smtpUsername: '',
		defaultMailbox: 'INBOX',
		sentMailbox: 'Sent',
		isConfigured: false,
		hasIMAPPassword: false,
		hasSMTPPassword: false
	};

	const emptyComposeDraft: ComposeDraft = {
		to: '',
		cc: '',
		bcc: '',
		subject: '',
		body: ''
	};

	const defaultMailboxes: Mailbox[] = [
		{ name: 'INBOX', displayName: 'Inbox', unseen: 0, total: 0 },
		{ name: 'Sent', displayName: 'Sent', unseen: 0, total: 0 },
		{ name: 'Drafts', displayName: 'Drafts', unseen: 0, total: 0 },
		{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 },
		{ name: 'Trash', displayName: 'Trash', unseen: 0, total: 0 }
	];

	let account = $state<MailAccount>(emptyAccount);
	let accountDraft = $state<MailAccountDraft>({ ...emptyAccount, imapPassword: '', smtpPassword: '' });
	let composeDraft = $state<ComposeDraft>(emptyComposeDraft);
	let mailboxes = $state<Mailbox[]>([]);
	let messages = $state<MailMessage[]>([]);
	let selectedMailbox = $state('INBOX');
	let selectedMessage = $state<MailMessage | null>(null);
	let searchText = $state('');
	let isUnreadOnly = $state(false);
	let isLoading = $state(false);
	let isLoadingMessage = $state(false);
	let isSavingAccount = $state(false);
	let isTestingAccount = $state(false);
	let isSending = $state(false);
	let isSettingsOpen = $state(false);
	let isComposeOpen = $state(false);
	let errorMessage = $state('');
	let settingsMessage = $state('');
	let composeMessage = $state('');

	const visibleMessages = () => messages.filter((message) => !isUnreadOnly || !message.isRead);
	const displayedMailboxes = () => (account.isConfigured && mailboxes.length ? mailboxes : defaultMailboxes);
	const selectedMessageBody = () => selectedMessage?.body || selectedMessage?.preview || '';

	onMount(loadMail);

	async function loadMail() {
		isLoading = true;
		errorMessage = '';
		try {
			await loadAccount();
			if (!account.isConfigured) {
				mailboxes = [];
				messages = [];
				selectedMessage = null;
				isSettingsOpen = true;
				return;
			}
			await loadMailboxes();
			await loadMessages();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Could not load mail.';
		} finally {
			isLoading = false;
		}
	}

	async function loadAccount() {
		const response = await fetch('/mail/api/account', { credentials: 'include' });
		if (!response.ok) throw new Error(await responseErrorMessage(response, 'Connect a mail account to start.'));
		account = { ...emptyAccount, ...((await response.json()) as Partial<MailAccount>) };
		selectedMailbox = selectedMailbox || account.defaultMailbox || 'INBOX';
		accountDraft = createAccountDraft(account);
	}

	async function loadMailboxes() {
		const response = await fetch('/mail/api/mailboxes', { credentials: 'include' });
		if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not load mailboxes.'));
		mailboxes = ((await response.json()) as { mailboxes?: Mailbox[] }).mailboxes ?? [];
	}

	async function loadMessages() {
		if (!account.isConfigured) return;
		const query = new URLSearchParams({ mailbox: selectedMailbox, limit: '50' });
		if (searchText.trim()) query.set('query', searchText.trim());
		errorMessage = '';
		try {
			const response = await fetch(`/mail/api/messages?${query}`, { credentials: 'include' });
			if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not load messages.'));
			messages = ((await response.json()) as { messages?: MailMessage[] }).messages ?? [];
			selectedMessage = visibleMessages()[0] ?? null;
			if (selectedMessage) await loadMessage(selectedMessage);
		} catch (error) {
			messages = [];
			selectedMessage = null;
			errorMessage = error instanceof Error ? error.message : 'Could not load messages.';
		}
	}

	async function loadMessage(message: MailMessage) {
		isLoadingMessage = true;
		errorMessage = '';
		try {
			const response = await fetch(`/mail/api/messages/${encodeURIComponent(message.mailbox)}/${message.uid}`, { credentials: 'include' });
			if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not load message.'));
			const detail = (await response.json()) as Partial<MailMessage>;
			selectedMessage = { ...message, ...detail };
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'Could not load message.';
		} finally {
			isLoadingMessage = false;
		}
	}

	function selectMailbox(mailboxName: string) {
		selectedMailbox = mailboxName;
		loadMessages();
	}

	function selectMessage(message: MailMessage) {
		selectedMessage = message;
		loadMessage(message);
	}

	function openSettings() {
		accountDraft = createAccountDraft(account);
		settingsMessage = '';
		isSettingsOpen = true;
	}

	function openCompose() {
		composeDraft = { ...emptyComposeDraft };
		composeMessage = '';
		isComposeOpen = true;
	}

	function openReply() {
		if (!selectedMessage) return;
		composeDraft = {
			to: selectedMessage.from,
			cc: '',
			bcc: '',
			subject: selectedMessage.subject.toLowerCase().startsWith('re:') ? selectedMessage.subject : `Re: ${selectedMessage.subject}`,
			body: ''
		};
		composeMessage = '';
		isComposeOpen = true;
	}

	async function saveAccount() {
		isSavingAccount = true;
		settingsMessage = '';
		try {
			const response = await fetch('/mail/api/account', {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(accountDraftPayload())
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not save account.'));
			account = { ...emptyAccount, ...((await response.json()) as Partial<MailAccount>) };
			accountDraft = createAccountDraft(account);
			settingsMessage = 'Settings saved.';
			await loadMail();
		} catch (error) {
			settingsMessage = error instanceof Error ? error.message : 'Save failed.';
		} finally {
			isSavingAccount = false;
		}
	}

	async function testAccount() {
		isTestingAccount = true;
		settingsMessage = '';
		try {
			const response = await fetch('/mail/api/account/test', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(accountDraftPayload())
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, 'Connection test failed.'));
			settingsMessage = 'IMAP and SMTP connection succeeded.';
		} catch (error) {
			settingsMessage = error instanceof Error ? error.message : 'Connection test failed.';
		} finally {
			isTestingAccount = false;
		}
	}

	async function sendMessage() {
		isSending = true;
		composeMessage = '';
		try {
			const response = await fetch('/mail/api/messages/send', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					to: splitAddressList(composeDraft.to),
					cc: splitAddressList(composeDraft.cc),
					bcc: splitAddressList(composeDraft.bcc),
					subject: composeDraft.subject,
					body: composeDraft.body
				})
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not send message.'));
			isComposeOpen = false;
			await loadMail();
		} catch (error) {
			composeMessage = error instanceof Error ? error.message : 'Could not send message.';
		} finally {
			isSending = false;
		}
	}

	async function moveSelectedMessage(targetHint: string) {
		if (!selectedMessage) return;
		const targetMailbox = mailboxByHint(targetHint);
		if (!targetMailbox) {
			errorMessage = `${targetHint} mailbox was not found.`;
			return;
		}
		const response = await fetch(`/mail/api/messages/${encodeURIComponent(selectedMessage.mailbox)}/${selectedMessage.uid}/move`, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ targetMailbox })
		});
		if (!response.ok) {
			errorMessage = await responseErrorMessage(response, 'Could not move message.');
			return;
		}
		await loadMessages();
	}

	async function toggleSelectedMessageRead() {
		if (!selectedMessage) return;
		const seen = !selectedMessage.isRead;
		const response = await fetch(`/mail/api/messages/${encodeURIComponent(selectedMessage.mailbox)}/${selectedMessage.uid}/flags`, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ seen })
		});
		if (!response.ok) {
			errorMessage = await responseErrorMessage(response, 'Could not update message.');
			return;
		}
		selectedMessage = { ...selectedMessage, isRead: seen };
		messages = messages.map((message) => (message.uid === selectedMessage?.uid ? { ...message, isRead: seen } : message));
	}

	function createAccountDraft(value: MailAccount): MailAccountDraft {
		return { ...emptyAccount, ...value, imapPassword: '', smtpPassword: '' };
	}

	function accountDraftPayload() {
		return {
			email: accountDraft.email,
			fromAddress: accountDraft.email,
			displayName: accountDraft.displayName,
			imapHost: accountDraft.imapHost,
			imapPort: Number(accountDraft.imapPort),
			imapSecurity: accountDraft.imapSecurity,
			imapUsername: accountDraft.imapUsername,
			imapPassword: accountDraft.imapPassword,
			smtpHost: accountDraft.smtpHost,
			smtpPort: Number(accountDraft.smtpPort),
			smtpSecurity: accountDraft.smtpSecurity,
			smtpUsername: accountDraft.smtpUsername,
			smtpPassword: accountDraft.smtpPassword,
			defaultMailbox: accountDraft.defaultMailbox,
			sentMailbox: accountDraft.sentMailbox
		};
	}

	function splitAddressList(value: string) {
		return value
			.split(',')
			.map((address) => address.trim())
			.filter(Boolean);
	}

	function mailboxByHint(hint: string) {
		const normalizedHint = hint.toLowerCase();
		return displayedMailboxes().find((mailbox) => mailbox.name.toLowerCase().includes(normalizedHint))?.name;
	}

	function mailboxIcon(mailboxName: string) {
		const normalizedMailboxName = mailboxName.toLowerCase();
		if (normalizedMailboxName.includes('sent')) return SendIcon;
		if (normalizedMailboxName.includes('draft')) return FileTextIcon;
		if (normalizedMailboxName.includes('archive')) return ArchiveIcon;
		if (normalizedMailboxName.includes('trash')) return Trash2Icon;
		return InboxIcon;
	}

	async function responseErrorMessage(response: Response, fallback: string) {
		const message = (await response.text()).trim();
		if (!message || isHTMLResponse(response, message)) return unavailableMessage(response, fallback);
		return message;
	}

	function isHTMLResponse(response: Response, message: string) {
		const contentType = response.headers.get('content-type')?.toLowerCase() ?? '';
		const normalizedMessage = message.toLowerCase();
		return contentType.includes('text/html') || normalizedMessage.startsWith('<!doctype html') || normalizedMessage.startsWith('<html');
	}

	function unavailableMessage(response: Response, fallback: string) {
		if (response.status >= 500) return `${fallback} Mail service is temporarily unavailable.`;
		return fallback;
	}
</script>

<svelte:head>
	<title>Mail · Intern Kim</title>
</svelte:head>

<main class="grid h-[calc(100svh-48px)] min-h-0 grid-cols-[240px_minmax(320px,380px)_minmax(0,1fr)] overflow-hidden bg-background text-foreground max-lg:grid-cols-[260px_minmax(0,1fr)] max-md:grid-cols-1">
	<aside class="flex min-h-0 flex-col border-r bg-sidebar text-sidebar-foreground max-md:hidden">
		<div class="flex h-14 items-center gap-2 border-b px-3">
			<div class="flex size-8 items-center justify-center rounded-md border bg-background">
				<MailIcon class="size-4" />
			</div>
			<div class="min-w-0 flex-1">
				<p class="truncate text-sm font-medium">Mail</p>
				<p class="truncate text-xs text-muted-foreground">{account.email || 'IMAP / SMTP'}</p>
			</div>
			<Button variant="ghost" size="icon-sm" aria-label="Mail settings" onclick={openSettings}>
				<SettingsIcon />
			</Button>
		</div>

		<div class="border-b p-3">
			<Button class="h-9 w-full justify-start gap-2" variant="secondary" onclick={openCompose} disabled={!account.isConfigured}>
				<PencilIcon />
				Compose
			</Button>
		</div>

		<nav class="min-h-0 flex-1 overflow-auto p-2">
			<p class="px-2 py-2 text-xs font-medium text-muted-foreground">Mailboxes</p>
			<div class="space-y-1">
				{#each displayedMailboxes() as mailbox (mailbox.name)}
					{@const Icon = mailboxIcon(mailbox.name)}
					<button
						type="button"
						class="flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50"
						data-active={mailbox.name === selectedMailbox}
						disabled={!account.isConfigured}
						onclick={() => selectMailbox(mailbox.name)}
					>
						<Icon class="size-4 shrink-0" />
						<span class="min-w-0 flex-1 truncate">{mailbox.displayName || mailbox.name}</span>
						{#if mailbox.unseen}
							<span class="rounded-md bg-primary px-1.5 py-0.5 text-[10px] font-medium text-primary-foreground">{mailbox.unseen}</span>
						{:else if mailbox.total}
							<span class="text-xs text-muted-foreground">{mailbox.total}</span>
						{/if}
					</button>
				{/each}
			</div>
		</nav>

		<div class="border-t p-3">
			<button type="button" class="w-full rounded-lg border bg-background p-3 text-left" onclick={openSettings}>
				<p class="text-xs font-medium">{account.isConfigured ? 'Connected account' : 'No account connected'}</p>
				<p class="mt-1 text-xs leading-5 text-muted-foreground">
					{account.isConfigured ? 'IMAP and SMTP settings are stored server-side.' : 'Add IMAP and SMTP settings to receive and send mail.'}
				</p>
			</button>
		</div>
	</aside>

	<section class="flex min-h-0 flex-col border-r bg-muted/20 max-md:border-r-0">
		<header class="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-3">
			<Button class="md:hidden" variant="ghost" size="icon-sm" aria-label="Mail settings" onclick={openSettings}>
				<PanelLeftIcon />
			</Button>
			<div class="min-w-0 flex-1">
				<p class="truncate text-sm font-semibold">{selectedMailbox}</p>
				<p class="truncate text-xs text-muted-foreground">{visibleMessages().length} messages</p>
			</div>
			<Button variant="ghost" size="icon-sm" aria-label="Refresh mail" onclick={loadMail} disabled={isLoading}>
				<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
			</Button>
		</header>

		<div class="border-b bg-background p-3">
			<div class="relative">
				<SearchIcon class="pointer-events-none absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
				<Input class="h-9 pl-8" bind:value={searchText} placeholder="Search mail" disabled={!account.isConfigured} onkeydown={(event) => event.key === 'Enter' && loadMessages()} />
			</div>
			<div class="mt-3 flex items-center justify-between">
				<p class="text-xs font-medium text-muted-foreground">{account.isConfigured ? selectedMailbox : 'Connect account'}</p>
				<div class="flex items-center gap-2">
					<Switch id="mail-unread-only" bind:checked={isUnreadOnly} />
					<Label for="mail-unread-only" class="text-xs text-muted-foreground">Unread</Label>
				</div>
			</div>
		</div>

		<div class="min-h-0 flex-1 overflow-auto p-2">
			{#if errorMessage}
				<div role="alert" class="mb-2 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
					<p class="font-medium">Mail needs attention</p>
					<p class="mt-1 line-clamp-3 text-xs leading-5">{errorMessage}</p>
				</div>
			{/if}

			<div class="space-y-2">
				{#each visibleMessages() as message (message.uid)}
					<button
						type="button"
						class="w-full rounded-lg border bg-background p-3 text-left shadow-xs transition-colors hover:bg-muted/60 data-[active=true]:border-primary/40 data-[active=true]:bg-background data-[active=true]:shadow-sm"
						data-active={selectedMessage?.uid === message.uid}
						onclick={() => selectMessage(message)}
					>
						<div class="flex items-center gap-2">
							{#if message.isRead}
								<MailOpenIcon class="size-4 text-muted-foreground" />
							{:else}
								<MailIcon class="size-4" />
							{/if}
							<span class="min-w-0 flex-1 truncate text-sm font-medium">{message.from || 'Unknown sender'}</span>
						</div>
						<p class="mt-2 truncate text-sm">{message.subject || '(No subject)'}</p>
						<p class="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">{message.preview}</p>
					</button>
				{/each}

				{#if !visibleMessages().length}
					<div class="rounded-lg border border-dashed bg-background p-8 text-center">
						<MailOpenIcon class="mx-auto size-5 text-muted-foreground" />
						<p class="mt-3 text-sm font-medium">{account.isConfigured ? 'No messages here' : 'Connect mail'}</p>
						<p class="mt-1 text-xs text-muted-foreground">{account.isConfigured ? 'Try another mailbox or search.' : 'Open settings and add IMAP / SMTP credentials.'}</p>
						{#if !account.isConfigured}
							<Button class="mt-4 gap-2" variant="secondary" onclick={openSettings}>
								<SettingsIcon />
								Mail settings
							</Button>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</section>

	<section class="flex min-h-0 flex-col bg-background max-lg:hidden">
		<header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<div class="min-w-0 flex-1">
				<p class="truncate text-sm text-muted-foreground">All Inboxes / {selectedMailbox}</p>
				<p class="truncate text-sm font-medium">{selectedMessage?.subject || 'Select a message'}</p>
			</div>
			<div class="flex items-center gap-1">
				<Button variant="ghost" size="icon-sm" aria-label="Archive" onclick={() => moveSelectedMessage('archive')} disabled={!selectedMessage}>
					<ArchiveIcon />
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label="Trash" onclick={() => moveSelectedMessage('trash')} disabled={!selectedMessage}>
					<Trash2Icon />
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label="Mark read" onclick={toggleSelectedMessageRead} disabled={!selectedMessage}>
					{#if selectedMessage?.isRead}
						<MailIcon />
					{:else}
						<MailOpenIcon />
					{/if}
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label="Reply" onclick={openReply} disabled={!selectedMessage}>
					<SendIcon />
				</Button>
			</div>
		</header>

		<div class="min-h-0 flex-1 overflow-auto p-6">
			{#if selectedMessage}
				<div class="mx-auto max-w-3xl space-y-4">
					<div>
						<p class="text-xs text-muted-foreground">{selectedMessage.from}</p>
						<h2 class="mt-2 text-2xl font-semibold tracking-tight">{selectedMessage.subject || '(No subject)'}</h2>
						{#if selectedMessage.to}
							<p class="mt-2 text-xs text-muted-foreground">To {selectedMessage.to}</p>
						{/if}
					</div>
					<Separator />
					{#if isLoadingMessage}
						<p class="text-sm text-muted-foreground">Loading message...</p>
					{:else}
						<p class="whitespace-pre-wrap text-sm leading-6">{selectedMessageBody()}</p>
					{/if}
				</div>
			{:else}
				<div class="flex h-full items-center justify-center">
					<div class="max-w-sm text-center">
						<div class="mx-auto flex size-12 items-center justify-center rounded-xl border bg-muted/50">
							<MailIcon class="size-5 text-muted-foreground" />
						</div>
						<h2 class="mt-4 text-lg font-semibold">{account.isConfigured ? 'Ready for mail' : 'Mail is not connected'}</h2>
						<p class="mt-2 text-sm leading-6 text-muted-foreground">{account.isConfigured ? 'Choose a message from the list.' : 'Connect an IMAP and SMTP account to receive and send mail.'}</p>
						<Button class="mt-4 gap-2" variant="secondary" onclick={account.isConfigured ? openCompose : openSettings}>
							{#if account.isConfigured}
								<PencilIcon />
								Compose
							{:else}
								<SettingsIcon />
								Mail settings
							{/if}
						</Button>
					</div>
				</div>
			{/if}
		</div>

		<div class="border-t p-4">
			<Button class="w-full justify-start gap-2" variant="outline" onclick={selectedMessage ? openReply : openCompose} disabled={!account.isConfigured}>
				<PencilIcon />
				{selectedMessage ? 'Reply...' : 'Compose...'}
			</Button>
		</div>
	</section>
</main>

<Sheet.Root bind:open={isSettingsOpen}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>Mail settings</Sheet.Title>
			<Sheet.Description>Connect one email account for receiving and sending mail.</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-5 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); saveAccount(); }}>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-email">Email address</Label>
					<Input id="mail-email" bind:value={accountDraft.email} placeholder="you@example.com" />
				</div>
				<div class="space-y-2">
					<Label for="mail-display-name">Display name</Label>
					<Input id="mail-display-name" bind:value={accountDraft.displayName} placeholder="Your name" />
				</div>
				<div class="space-y-2">
					<Label for="mail-default-mailbox">Default mailbox</Label>
					<Input id="mail-default-mailbox" bind:value={accountDraft.defaultMailbox} />
				</div>
			</div>

			<Separator />

			<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_96px_120px]">
				<div class="space-y-2">
					<Label for="mail-imap-host">IMAP host</Label>
					<Input id="mail-imap-host" bind:value={accountDraft.imapHost} placeholder="imap.gmail.com" />
				</div>
				<div class="space-y-2">
					<Label for="mail-imap-port">Port</Label>
					<Input id="mail-imap-port" type="number" bind:value={accountDraft.imapPort} />
				</div>
				<div class="space-y-2">
					<Label for="mail-imap-security">Security</Label>
					<select id="mail-imap-security" class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm" bind:value={accountDraft.imapSecurity}>
						<option value="tls">SSL/TLS</option>
						<option value="starttls">STARTTLS</option>
						<option value="none">None</option>
					</select>
				</div>
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-imap-user">IMAP username</Label>
					<Input id="mail-imap-user" bind:value={accountDraft.imapUsername} />
				</div>
				<div class="space-y-2">
					<Label for="mail-imap-password">IMAP password</Label>
					<Input id="mail-imap-password" type="password" bind:value={accountDraft.imapPassword} placeholder={account.hasIMAPPassword ? 'Saved password' : 'App password'} />
				</div>
			</div>

			<Separator />

			<div class="grid gap-3 sm:grid-cols-[minmax(0,1fr)_96px_120px]">
				<div class="space-y-2">
					<Label for="mail-smtp-host">SMTP host</Label>
					<Input id="mail-smtp-host" bind:value={accountDraft.smtpHost} placeholder="smtp.gmail.com" />
				</div>
				<div class="space-y-2">
					<Label for="mail-smtp-port">Port</Label>
					<Input id="mail-smtp-port" type="number" bind:value={accountDraft.smtpPort} />
				</div>
				<div class="space-y-2">
					<Label for="mail-smtp-security">Security</Label>
					<select id="mail-smtp-security" class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm" bind:value={accountDraft.smtpSecurity}>
						<option value="tls">SSL/TLS</option>
						<option value="starttls">STARTTLS/TLS</option>
						<option value="none">None</option>
					</select>
				</div>
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-smtp-user">SMTP username</Label>
					<Input id="mail-smtp-user" bind:value={accountDraft.smtpUsername} />
				</div>
				<div class="space-y-2">
					<Label for="mail-smtp-password">SMTP password</Label>
					<Input id="mail-smtp-password" type="password" bind:value={accountDraft.smtpPassword} placeholder={account.hasSMTPPassword ? 'Saved password' : 'App password'} />
				</div>
				<div class="space-y-2">
					<Label for="mail-sent-mailbox">Sent mailbox</Label>
					<Input id="mail-sent-mailbox" bind:value={accountDraft.sentMailbox} />
				</div>
			</div>

			{#if settingsMessage}
				<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{settingsMessage}</p>
			{/if}

			<Sheet.Footer class="gap-2 sm:justify-between">
				<Button type="button" variant="outline" onclick={testAccount} disabled={isTestingAccount || isSavingAccount}>
					{isTestingAccount ? 'Testing...' : 'Test connection'}
				</Button>
				<Button type="submit" disabled={isSavingAccount || isTestingAccount}>
					{isSavingAccount ? 'Saving...' : 'Save settings'}
				</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>

<Sheet.Root bind:open={isComposeOpen}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-2xl">
		<Sheet.Header>
			<Sheet.Title>Compose</Sheet.Title>
			<Sheet.Description>{account.fromAddress || account.email}</Sheet.Description>
		</Sheet.Header>
		<form class="grid gap-4 px-4 pb-4" onsubmit={(event) => { event.preventDefault(); sendMessage(); }}>
			<div class="space-y-2">
				<Label for="mail-compose-to">To</Label>
				<Input id="mail-compose-to" bind:value={composeDraft.to} placeholder="name@example.com" />
			</div>
			<div class="grid gap-3 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="mail-compose-cc">CC</Label>
					<Input id="mail-compose-cc" bind:value={composeDraft.cc} />
				</div>
				<div class="space-y-2">
					<Label for="mail-compose-bcc">BCC</Label>
					<Input id="mail-compose-bcc" bind:value={composeDraft.bcc} />
				</div>
			</div>
			<div class="space-y-2">
				<Label for="mail-compose-subject">Subject</Label>
				<Input id="mail-compose-subject" bind:value={composeDraft.subject} />
			</div>
			<div class="space-y-2">
				<Label for="mail-compose-body">Body</Label>
				<Textarea id="mail-compose-body" class="min-h-72 resize-none" bind:value={composeDraft.body} />
			</div>
			{#if composeMessage}
				<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{composeMessage}</p>
			{/if}
			<Sheet.Footer>
				<Button type="submit" class="gap-2" disabled={isSending || !composeDraft.to.trim() || (!composeDraft.subject.trim() && !composeDraft.body.trim())}>
					<SendIcon />
					{isSending ? 'Sending...' : 'Send'}
				</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
