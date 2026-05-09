<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
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
		hasIMAPPassword: boolean;
		hasSMTPPassword: boolean;
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
		date: string;
		preview: string;
		isRead: boolean;
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
		hasIMAPPassword: false,
		hasSMTPPassword: false
	};

	let account = $state<MailAccount>(emptyAccount);
	let mailboxes = $state<Mailbox[]>([]);
	let messages = $state<MailMessage[]>([]);
	let selectedMailbox = $state('INBOX');
	let selectedMessage = $state<MailMessage | null>(null);
	let searchText = $state('');
	let isUnreadOnly = $state(false);
	let isLoading = $state(false);
	let errorMessage = $state('');
	const visibleMessages = () => messages.filter((message) => !isUnreadOnly || !message.isRead);
	const displayedMailboxes = () => (mailboxes.length ? mailboxes : defaultMailboxes);
	const hasConnectedAccount = () => Boolean(account.email || account.imapHost || account.smtpHost);

	const defaultMailboxes: Mailbox[] = [
		{ name: 'INBOX', displayName: 'Inbox', unseen: 0, total: 0 },
		{ name: 'Sent', displayName: 'Sent', unseen: 0, total: 0 },
		{ name: 'Drafts', displayName: 'Drafts', unseen: 0, total: 0 },
		{ name: 'Archive', displayName: 'Archive', unseen: 0, total: 0 },
		{ name: 'Trash', displayName: 'Trash', unseen: 0, total: 0 }
	];

	onMount(loadMail);

	async function loadMail() {
		isLoading = true;
		errorMessage = '';
		try {
			await loadAccount();
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
		selectedMailbox = account.defaultMailbox || 'INBOX';
	}

	async function loadMailboxes() {
		const response = await fetch('/mail/api/mailboxes', { credentials: 'include' });
		if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not load mailboxes.'));
		mailboxes = ((await response.json()) as { mailboxes?: Mailbox[] }).mailboxes ?? [];
	}

	async function loadMessages() {
		const query = new URLSearchParams({
			mailbox: selectedMailbox,
			limit: '50'
		});
		if (searchText.trim()) query.set('query', searchText.trim());
		const response = await fetch(`/mail/api/messages?${query}`, { credentials: 'include' });
		if (!response.ok) throw new Error(await responseErrorMessage(response, 'Could not load messages.'));
		messages = ((await response.json()) as { messages?: MailMessage[] }).messages ?? [];
		selectedMessage = visibleMessages()[0] ?? null;
	}

	function selectMailbox(mailboxName: string) {
		selectedMailbox = mailboxName;
		loadMessages();
	}

	function selectMessage(message: MailMessage) {
		selectedMessage = message;
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
		if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
		return message;
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
			<Button variant="ghost" size="icon-sm" aria-label="Mail settings">
				<SettingsIcon />
			</Button>
		</div>

		<div class="border-b p-3">
			<Button class="h-9 w-full justify-start gap-2" variant="secondary">
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
						class="flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground"
						data-active={mailbox.name === selectedMailbox}
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
			<div class="rounded-lg border bg-background p-3">
				<p class="text-xs font-medium">{hasConnectedAccount() ? 'Connected account' : 'No account connected'}</p>
				<p class="mt-1 text-xs leading-5 text-muted-foreground">
					{hasConnectedAccount() ? 'IMAP and SMTP settings are stored server-side.' : 'Add IMAP and SMTP settings to receive and send mail.'}
				</p>
			</div>
		</div>
	</aside>

	<section class="flex min-h-0 flex-col border-r bg-muted/20 max-md:border-r-0">
		<header class="flex h-14 shrink-0 items-center gap-2 border-b bg-background px-3">
			<Button class="md:hidden" variant="ghost" size="icon-sm" aria-label="Mailboxes">
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
				<Input class="h-9 pl-8" bind:value={searchText} placeholder="Search mail" onkeydown={(event) => event.key === 'Enter' && loadMessages()} />
			</div>
			<div class="mt-3 flex items-center justify-between">
				<p class="text-xs font-medium text-muted-foreground">Inbox</p>
				<div class="flex items-center gap-2">
					<Switch id="mail-unread-only" bind:checked={isUnreadOnly} />
					<Label for="mail-unread-only" class="text-xs text-muted-foreground">Unread</Label>
				</div>
			</div>
		</div>

		<div class="min-h-0 flex-1 overflow-auto p-2">
			{#if errorMessage}
				<div class="mb-2 rounded-lg border border-border bg-background p-3 text-sm">
					<p class="font-medium">Mail is not connected</p>
					<p class="mt-1 text-xs leading-5 text-muted-foreground">{errorMessage}</p>
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
						<p class="mt-3 text-sm font-medium">No messages here</p>
						<p class="mt-1 text-xs text-muted-foreground">Messages will appear after the account is connected.</p>
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
				<Button variant="ghost" size="icon-sm" aria-label="Archive">
					<ArchiveIcon />
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label="Trash">
					<Trash2Icon />
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label="Send">
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
					</div>
					<Separator />
					<p class="whitespace-pre-wrap text-sm leading-6">{selectedMessage.preview}</p>
				</div>
			{:else}
				<div class="flex h-full items-center justify-center">
					<div class="max-w-sm text-center">
						<div class="mx-auto flex size-12 items-center justify-center rounded-xl border bg-muted/50">
							<MailIcon class="size-5 text-muted-foreground" />
						</div>
						<h2 class="mt-4 text-lg font-semibold">Ready for mail</h2>
						<p class="mt-2 text-sm leading-6 text-muted-foreground">Connect an IMAP and SMTP account, then choose a message from the list.</p>
						<Button class="mt-4 gap-2" variant="secondary">
							<SettingsIcon />
							Mail settings
						</Button>
					</div>
				</div>
			{/if}
		</div>

		<div class="border-t p-4">
			<Textarea class="min-h-24 resize-none" placeholder="Reply..." />
		</div>
	</section>
</main>
