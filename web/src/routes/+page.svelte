<script lang="ts">
	import * as Chat from '$lib/components/ui/chat';

	import * as InputGroup from '$lib/components/ui/input-group';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Badge } from '$lib/components/ui/badge';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import XIcon from '@lucide/svelte/icons/x';
	const logoSrc = '/logo.svg';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import MailIcon from '@lucide/svelte/icons/mail';
	import FileIcon from '@lucide/svelte/icons/file';
	import DownloadIcon from '@lucide/svelte/icons/download';
	import { Separator } from '$lib/components/ui/separator';
	import { onMount } from 'svelte';
	import { marked } from 'marked';
	import DOMPurify from 'dompurify';

	type Message = {
		role: 'user' | 'assistant';
		content: string;
		imageUrl?: string;
	};

	const apiBase = () => (location.port === '5173' ? 'http://192.168.0.141:8080' : '');

	async function fetchCurrentUserEmail(): Promise<string> {
		try {
			const response = await fetch(`${apiBase()}/pico/me`);
			if (!response.ok) return '';
			const data = await response.json();
			return data.email || '';
		} catch {
			return '';
		}
	}

	let currentUser = $state('');
	let messages = $state<Message[]>([]);
	let input = $state('');
	let loading = $state(false);
	let ws = $state<WebSocket | null>(null);
	let sessionId = $state('');
	let pendingResolve: ((value: string) => void) | null = null;
	let currentContent = $state('');

	// Model
	let currentModel = $state('');
	let modelInput = $state('');
	let showModelDialog = $state(false);
	let modelSaving = $state(false);

	// Settings sheet
	let showSettingsSheet = $state(false);
	let userEmails = $state<string[]>([]);
	let newEmail = $state('');
	let emailLoading = $state(false);
	let adminEmail = $state('');

	const deviceId = () => {
		const host = location.hostname;
		const parts = host.split('.');
		return parts.length >= 3 ? parts[0] : '';
	};

	const pagesApi = () => {
		const id = deviceId();
		return id ? `https://api.example.test/api` : '';
	};

	async function loadUsers() {
		const api = pagesApi();
		const id = deviceId();
		if (!api || !id) return;
		try {
			const res = await fetch(`${api}/users?device_id=${id}`);
			if (res.ok) {
				const data = await res.json();
				userEmails = data.users || [];
				if (userEmails.length > 0 && !adminEmail) {
					adminEmail = userEmails[0];
				}
			}
		} catch { /* ignore */ }
	}

	async function addEmail() {
		const email = newEmail.trim().toLowerCase();
		if (!email) return;
		emailLoading = true;
		try {
			const res = await fetch(`${pagesApi()}/users`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ device_id: deviceId(), email })
			});
			if (res.ok) {
				const data = await res.json();
				userEmails = data.users || [];
				newEmail = '';
			}
		} catch { /* ignore */ }
		finally { emailLoading = false; }
	}

	async function removeEmail(email: string) {
		emailLoading = true;
		try {
			const res = await fetch(`${pagesApi()}/users/${encodeURIComponent(email)}?device_id=${deviceId()}`, {
				method: 'DELETE'
			});
			if (res.ok) {
				const data = await res.json();
				userEmails = data.users || [];
			}
		} catch { /* ignore */ }
		finally { emailLoading = false; }
	}

	// Image attachment
	let fileInput: HTMLInputElement;
	let attachedImage = $state<{ file: File; preview: string } | null>(null);

	const shortModel = $derived(() => {
		const parts = currentModel.split('/');
		return parts[parts.length - 1] || currentModel;
	});

	const imageExtensions = new Set(['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg']);

	function isFileLink(href: string): boolean {
		return href.startsWith('/pico/files/');
	}

	function getFileExtension(path: string): string {
		return path.split('.').pop()?.toLowerCase() || '';
	}

	function getFileName(path: string): string {
		return path.split('/').pop() || path;
	}

	function renderMarkdown(text: string): string {
		const renderer = new marked.Renderer();

		renderer.image = ({ href, title, text: alt }) => {
			if (isFileLink(href)) {
				return `<a href="${href}" download class="file-preview file-preview-image" target="_blank"><img src="${href}" alt="${alt || ''}" /></a>`;
			}
			return `<img src="${href}" alt="${alt || ''}" title="${title || ''}" />`;
		};

		renderer.link = ({ href, title, text: linkText }) => {
			if (isFileLink(href)) {
				const extension = getFileExtension(href);
				const fileName = getFileName(href);
				if (imageExtensions.has(extension)) {
					return `<a href="${href}" download class="file-preview file-preview-image" target="_blank"><img src="${href}" alt="${fileName}" /></a>`;
				}
				return `<a href="${href}" download class="file-preview file-preview-doc" target="_blank"><span class="file-icon">📄</span><span class="file-info"><span class="file-name">${fileName}</span><span class="file-ext">${extension.toUpperCase()}</span></span><span class="file-action">↓</span></a>`;
			}
			return `<a href="${href}" title="${title || ''}" target="_blank" rel="noopener">${linkText}</a>`;
		};

		const raw = marked.parse(text, { async: false, renderer }) as string;
		return DOMPurify.sanitize(raw);
	}

	function handleFileSelect(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file || !file.type.startsWith('image/')) return;
		attachedImage = { file, preview: URL.createObjectURL(file) };
	}

	function removeImage() {
		if (attachedImage) URL.revokeObjectURL(attachedImage.preview);
		attachedImage = null;
		if (fileInput) fileInput.value = '';
	}

	async function loadModel() {
		try {
			const res = await fetch(`${apiBase()}/pico/model`);
			const data = await res.json();
			currentModel = data.model || '';
		} catch {
			/* ignore */
		}
	}

	async function saveModel() {
		if (!modelInput.trim() || modelInput === currentModel) {
			showModelDialog = false;
			return;
		}
		modelSaving = true;
		try {
			const res = await fetch(`${apiBase()}/pico/model`, {
				method: 'PUT',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ model: modelInput.trim() })
			});
			if (res.ok) {
				const data = await res.json();
				currentModel = data.new;
				if (ws) {
					ws.close();
					ws = null;
				}
			}
		} catch {
			/* ignore */
		} finally {
			modelSaving = false;
			showModelDialog = false;
		}
	}

	async function loadHistory() {
		if (!sessionId) return;
		try {
			const res = await fetch(`${apiBase()}/pico/history/${sessionId}`);
			if (res.ok) {
				const data = await res.json();
				if (data.messages?.length) {
					messages = data.messages;
				}
			}
		} catch { /* ignore */ }
		loading = false;
	}

	onMount(async () => {
		currentUser = await fetchCurrentUserEmail();
		sessionId = currentUser
			? `user-${currentUser.replace(/[^a-z0-9]/gi, '-')}`
			: `chat-${crypto.randomUUID().slice(0, 8)}`;
		loadHistory();
		loadModel();
		loadUsers();
	});

	async function getToken(): Promise<{ token: string; ws_url: string }> {
		const res = await fetch(`${apiBase()}/pico/token`);
		return res.json();
	}

	async function connectWs() {
		const { token } = await getToken();
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
		const host = location.port === '5173'
			? `${location.hostname}:8080`
			: location.host;
		const url = `${proto}://${host}/pico/ws?session_id=${sessionId}`;

		return new Promise<WebSocket>((resolve, reject) => {
			const socket = new WebSocket(url, [`token.${token}`]);
			socket.onopen = () => resolve(socket);
			socket.onerror = () => reject(new Error('WebSocket connection failed'));
			socket.onmessage = (e) => handlePicoMessage(JSON.parse(e.data));
			socket.onclose = () => {
				ws = null;
			};
		});
	}

	function handlePicoMessage(data: { type: string; payload?: Record<string, unknown> }) {
		switch (data.type) {
			case 'typing.start':
				loading = true;
				break;
			case 'typing.stop':
				break;
			case 'message.create': {
				const content = (data.payload?.content as string) || '';
				if (pendingResolve) {
					pendingResolve(content);
					pendingResolve = null;
				}
				break;
			}
			case 'message.update': {
				currentContent = (data.payload?.content as string) || '';
				break;
			}
			case 'error': {
				const msg = (data.payload?.message as string) || 'Unknown error';
				if (pendingResolve) {
					pendingResolve(`Error: ${msg}`);
					pendingResolve = null;
				}
				break;
			}
		}
	}

	async function fileToBase64(file: File): Promise<string> {
		return new Promise((resolve) => {
			const reader = new FileReader();
			reader.onload = () => resolve(reader.result as string);
			reader.readAsDataURL(file);
		});
	}

	async function send() {
		const text = input.trim();
		if (!text && !attachedImage) return;
		if (loading) return;

		const image = attachedImage;
		const imageDataUrl = image ? await fileToBase64(image.file) : undefined;
		messages.push({ role: 'user', content: text, imageUrl: imageDataUrl });
		input = '';
		removeImage();
		loading = true;
		currentContent = '';

		try {
			if (!ws || ws.readyState !== WebSocket.OPEN) {
				ws = await connectWs();
			}

			const payload: Record<string, unknown> = { content: text };
			if (imageDataUrl) {
				payload.media = [imageDataUrl];
			}

			const content = await new Promise<string>((resolve) => {
				pendingResolve = resolve;
				ws!.send(
					JSON.stringify({
						type: 'message.send',
						id: crypto.randomUUID(),
						payload
					})
				);
				setTimeout(() => {
					if (pendingResolve === resolve) {
						resolve(currentContent || '(timeout)');
						pendingResolve = null;
					}
				}, 120000);
			});

			messages.push({ role: 'assistant', content });
		} catch (e) {
			messages.push({ role: 'assistant', content: `Connection error: ${e}` });
		} finally {
			loading = false;
		}
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			send();
		}
	}
</script>

<svelte:head>
	<title>intern kim</title>
</svelte:head>

<!-- Settings Sheet -->
<Sheet.Root bind:open={showSettingsSheet}>
	<Sheet.Content side="right" class="flex flex-col p-6">
		<Sheet.Header class="px-0">
			<Sheet.Title>Settings</Sheet.Title>
		</Sheet.Header>
		<div class="flex flex-1 flex-col gap-6 overflow-y-auto p-1 -m-1 pt-4">
			{#if currentUser}
				<section class="flex flex-col gap-1.5">
					<h3 class="text-muted-foreground text-xs font-medium uppercase tracking-wider">Account</h3>
					<p class="text-sm">{currentUser}</p>
				</section>
				<Separator />
			{/if}

			<section class="flex flex-col gap-3">
				<div class="flex items-center justify-between">
					<h3 class="text-muted-foreground text-xs font-medium uppercase tracking-wider">Allowed Users</h3>
					<span class="text-muted-foreground text-xs">{userEmails.length}</span>
				</div>
				{#if userEmails.length > 0}
					<div class="flex flex-col gap-1">
						{#each userEmails as email, i}
							<div class="group flex items-center justify-between rounded-md px-2 py-1.5 transition-colors hover:bg-muted/50">
								<div class="flex items-center gap-2 text-sm">
									<span>{email}</span>
									{#if i === 0}
										<Badge variant="secondary" class="text-[10px]">admin</Badge>
									{/if}
								</div>
								{#if i > 0}
									<button
										onclick={() => removeEmail(email)}
										disabled={emailLoading}
										class="text-muted-foreground hover:text-destructive cursor-pointer opacity-0 transition-all group-hover:opacity-100"
									>
										<XIcon class="size-3.5" />
									</button>
								{/if}
							</div>
						{/each}
					</div>
				{:else}
					<p class="text-muted-foreground text-xs">No users configured.</p>
				{/if}
				<form
					onsubmit={(e) => {
						e.preventDefault();
						addEmail();
					}}
					
				>
					<ButtonGroup.Root class="flex w-full">
						<InputGroup.Root class="flex-1">
							<InputGroup.Addon>
								<MailIcon />
							</InputGroup.Addon>
							<InputGroup.Input
								bind:value={newEmail}
								type="email"
								placeholder="Add email..."
							/>
						</InputGroup.Root>
						<Button type="submit" size="icon" disabled={emailLoading || !newEmail.trim()}>
							{#if emailLoading}
								<LoaderIcon class="size-4 animate-spin" />
							{:else}
								<PlusIcon class="size-4" />
							{/if}
						</Button>
					</ButtonGroup.Root>
				</form>
			</section>
		</div>
	</Sheet.Content>
</Sheet.Root>

<!-- Model Dialog -->
<Dialog.Root bind:open={showModelDialog}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Change Model</Dialog.Title>
			<Dialog.Description>Enter an OpenRouter model ID.</Dialog.Description>
		</Dialog.Header>
		<div class="flex flex-col gap-3 py-4">
			<input
				bind:value={modelInput}
				onkeydown={(e) => {
					if (e.key === 'Enter') {
						e.preventDefault();
						saveModel();
					}
				}}
				placeholder="e.g. google/gemini-3.1-flash-lite-preview"
				class="border-input bg-background w-full rounded-lg border px-3 py-2 text-sm outline-none"
			/>
			{#if currentModel}
				<p class="text-muted-foreground text-xs">
					Current: {currentModel}
				</p>
			{/if}
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (showModelDialog = false)}>Cancel</Button>
			<Button onclick={saveModel} disabled={modelSaving}>
				{#if modelSaving}
					<LoaderIcon class="animate-spin" />
				{:else}
					Save
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<div class="flex h-svh flex-col">
	<!-- Header -->
	<div class="bg-background flex items-center justify-between border-b p-2">
		<div class="flex items-center gap-2 pl-1">
			<img src={logoSrc} alt="intern kim" class="size-8" />
			<div class="flex flex-col">
				<span class="text-sm font-medium leading-tight">intern kim</span>
				{#if currentUser}
					<span class="text-muted-foreground text-[10px] leading-tight">{currentUser}</span>
				{/if}
			</div>
		</div>
		<Button variant="ghost" size="icon" onclick={() => (showSettingsSheet = true)}>
			<SettingsIcon />
		</Button>
	</div>

	<!-- Chat area -->
	<div class="min-h-0 flex-1 overflow-hidden">
		<div class="mx-auto h-full max-w-2xl">
			<Chat.List class="h-full">
				{#if messages.length === 0}
					<div class="text-muted-foreground flex h-full items-center justify-center">
						<p>Send a message to start chatting.</p>
					</div>
				{/if}

				{#each messages as msg}
					<Chat.Bubble variant={msg.role === 'user' ? 'sent' : 'received'}>
						{#if msg.role === 'assistant'}
							<Chat.BubbleAvatar>
								<Chat.BubbleAvatarImage src={logoSrc} alt="intern kim" />
							</Chat.BubbleAvatar>
						{/if}
						<Chat.BubbleMessage>
							{#if msg.role === 'user'}
								{#if msg.imageUrl}
									<img
										src={msg.imageUrl}
										alt="Sent image"
										class="mb-1.5 max-h-48 rounded-lg object-contain"
									/>
								{/if}
								{#if msg.content}
									<span>{msg.content}</span>
								{/if}
							{:else}
								<div class="markdown max-w-none text-sm">
									{@html renderMarkdown(msg.content)}
								</div>
							{/if}
						</Chat.BubbleMessage>
					</Chat.Bubble>
				{/each}

				{#if loading}
					<Chat.Bubble variant="received">
						<Chat.BubbleAvatar>
							<Chat.BubbleAvatarImage src={logoSrc} alt="intern kim" />
						</Chat.BubbleAvatar>
						<Chat.BubbleMessage typing />
					</Chat.Bubble>
				{/if}
			</Chat.List>
		</div>
	</div>

	<!-- Input area -->
	<div class="mx-auto w-full max-w-2xl px-3 pb-3 pt-1">
		<input
			bind:this={fileInput}
			type="file"
			accept="image/*"
			class="hidden"
			onchange={handleFileSelect}
		/>
		{#if attachedImage}
			<div class="mb-2 flex items-center gap-2">
				<div class="relative">
					<img
						src={attachedImage.preview}
						alt="Attached"
						class="h-16 w-16 rounded-lg border object-cover"
					/>
					<button
						onclick={removeImage}
						class="bg-background border-border absolute -top-1.5 -right-1.5 flex size-5 items-center justify-center rounded-full border text-xs"
					>
						&times;
					</button>
				</div>
			</div>
		{/if}
		<InputGroup.Root>
			<InputGroup.Textarea
				bind:value={input}
				{onkeydown}
				placeholder="Type a message..."
				rows={1}
				class="min-h-0 resize-none"
			/>
			<InputGroup.Addon align="block-end">
				<InputGroup.Button
					variant="outline"
					class="rounded-full"
					size="icon-xs"
					onclick={() => fileInput.click()}
				>
					<PlusIcon />
				</InputGroup.Button>
				<button
					onclick={() => {
						modelInput = currentModel;
						showModelDialog = true;
					}}
					class="text-muted-foreground hover:text-foreground ms-auto cursor-pointer text-xs transition-colors"
				>
					{shortModel()}
				</button>
				<InputGroup.Button
					variant="default"
					class="rounded-full"
					size="icon-xs"
					disabled={loading || (!input.trim() && !attachedImage)}
					onclick={send}
				>
					<ArrowUpIcon />
					<span class="sr-only">Send</span>
				</InputGroup.Button>
			</InputGroup.Addon>
		</InputGroup.Root>
	</div>
</div>
