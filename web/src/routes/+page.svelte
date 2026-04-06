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
	import MaximizeIcon from '@lucide/svelte/icons/maximize-2';
	import QrCode from 'svelte-qrcode';
	import { Separator } from '$lib/components/ui/separator';
	import * as Code from '$lib/components/ui/code';
	import type { SupportedLanguage } from '$lib/components/ui/code/shiki';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { onMount } from 'svelte';
	import { parseMarkdownSegments } from '$lib/markdown';

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

	// Code fullscreen
	let codeFullscreen = $state(false);
	let codeFullscreenContent = $state('');
	let codeFullscreenLanguage = $state<SupportedLanguage | undefined>(undefined);

	// Image viewer
	let imageViewerOpen = $state(false);
	let imageViewerSource = $state('');
	let imageMenuOpen = $state(false);
	let imageMenuPosition = $state({ x: 0, y: 0 });
	let longPressTimer: ReturnType<typeof setTimeout> | null = null;

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
		return id ? `https://api.intern.kim/api` : '';
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

	function openCodeFullscreen(code: string, lang: string) {
		codeFullscreenContent = code;
		codeFullscreenLanguage = lang as SupportedLanguage | undefined;
		codeFullscreen = true;
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

		document.addEventListener('click', (e) => {
			const expandableImage = (e.target as HTMLElement).closest('.expandable-image') as HTMLElement | null;
			if (expandableImage) {
				e.preventDefault();
				const source = expandableImage.dataset.src || expandableImage.querySelector('img')?.src || '';
				if (source) openImageViewer(source);
			}
		});
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

	function openImageViewer(source: string) {
		imageViewerSource = source;
		imageViewerOpen = true;
	}

	function handleImageLongPressStart(event: TouchEvent | MouseEvent, source: string) {
		longPressTimer = setTimeout(() => {
			event.preventDefault();
			const { clientX, clientY } = 'touches' in event ? event.touches[0] : event;
			imageMenuPosition = { x: clientX, y: clientY };
			imageViewerSource = source;
			imageMenuOpen = true;
			longPressTimer = null;
		}, 500);
	}

	function handleImageLongPressEnd() {
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	}

	async function saveImage() {
		imageMenuOpen = false;
		const link = document.createElement('a');
		link.href = imageViewerSource;
		link.download = imageViewerSource.split('/').pop() || 'image';
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);
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
			{#if deviceId()}
				<section class="flex flex-col items-center gap-3">
					<QrCode value={`https://${deviceId()}.intern.kim`} size="180" />
					<p class="text-muted-foreground text-xs">{deviceId()}.intern.kim</p>
				</section>
				<Separator />
			{/if}

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

<!-- Image Viewer Dialog -->
<Dialog.Root bind:open={imageViewerOpen}>
	<Dialog.Content class="max-w-[90vw] max-h-[90vh] flex flex-col items-center p-2 sm:p-4">
		<Dialog.Header class="sr-only">
			<Dialog.Title>Image</Dialog.Title>
		</Dialog.Header>
		{#if imageViewerSource}
			<img
				src={imageViewerSource}
				alt="Expanded"
				class="max-h-[80vh] max-w-full rounded-md object-contain"
			/>
		{/if}
		<div class="mt-2 flex gap-2">
			<Button variant="ghost" size="sm" class="gap-1.5" onclick={saveImage}>
				<DownloadIcon class="size-3.5" />
				Save
			</Button>
		</div>
	</Dialog.Content>
</Dialog.Root>

<!-- Image Long-Press Menu -->
{#if imageMenuOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50"
		onclick={() => (imageMenuOpen = false)}
		oncontextmenu={(e) => { e.preventDefault(); imageMenuOpen = false; }}
	>
		<div
			class="bg-popover text-popover-foreground absolute rounded-lg border p-1 shadow-md"
			style="left: {imageMenuPosition.x}px; top: {imageMenuPosition.y}px; transform: translate(-50%, -100%);"
		>
			<button
				class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-sm cursor-pointer"
				onclick={saveImage}
			>
				<DownloadIcon class="size-3.5" />
				Save Image
			</button>
			<button
				class="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-sm cursor-pointer"
				onclick={() => { imageMenuOpen = false; openImageViewer(imageViewerSource); }}
			>
				<MaximizeIcon class="size-3.5" />
				Expand
			</button>
		</div>
	</div>
{/if}

<!-- Code Fullscreen Dialog -->
<Dialog.Root bind:open={codeFullscreen}>
	<Dialog.Content class="max-w-3xl max-h-[80vh] flex flex-col">
		<Dialog.Header>
			<Dialog.Title>{codeFullscreenLanguage || 'Code'}</Dialog.Title>
		</Dialog.Header>
		<div class="flex-1 overflow-auto">
			<Code.Root code={codeFullscreenContent} lang={codeFullscreenLanguage} class="border-0">
				<Code.CopyButton />
			</Code.Root>
		</div>
	</Dialog.Content>
</Dialog.Root>

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
				{#if messages.length === 0 && loading}
					<div class="flex flex-col gap-4 p-4">
						{#each [1, 2] as _}
							<div class="flex items-start gap-3">
								<div class="bg-muted size-8 shrink-0 animate-pulse rounded-full"></div>
								<div class="flex flex-col gap-2">
									<div class="bg-muted h-4 w-48 animate-pulse rounded"></div>
									<div class="bg-muted h-4 w-64 animate-pulse rounded"></div>
									<div class="bg-muted h-4 w-40 animate-pulse rounded"></div>
								</div>
							</div>
						{/each}
					</div>
				{/if}

				{#each messages as msg}
					<Chat.Bubble variant={msg.role === 'user' ? 'sent' : 'received'}>
						<Chat.BubbleMessage>
							{#if msg.role === 'user'}
								{#if msg.imageUrl}
									<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
									<img
										src={msg.imageUrl}
										alt="Attached"
										class="mb-1.5 max-h-48 cursor-pointer rounded-lg object-contain"
										onclick={() => openImageViewer(msg.imageUrl!)}
										ontouchstart={(e) => handleImageLongPressStart(e, msg.imageUrl!)}
										ontouchend={handleImageLongPressEnd}
										oncontextmenu={(e) => {
											e.preventDefault();
											imageMenuPosition = { x: e.clientX, y: e.clientY };
											imageViewerSource = msg.imageUrl!;
											imageMenuOpen = true;
										}}
									/>
								{/if}
								{#if msg.content}
									<span>{msg.content}</span>
								{/if}
							{:else}
								<div class="markdown text-sm">
									{#each parseMarkdownSegments(msg.content) as segment}
										{#if segment.type === 'html'}
											{@html segment.content}
										{:else}
											<div class="my-2 overflow-hidden rounded-lg border">
												<div class="bg-muted flex items-center justify-between border-b pl-2.5 pr-0 py-1">
													<span class="text-muted-foreground font-mono text-xs">{segment.lang || 'text'}</span>
													<div class="flex items-center">
														<CopyButton text={segment.code} variant="ghost" size="icon" />
														<Button variant="ghost" size="icon" onclick={() => openCodeFullscreen(segment.code, segment.lang)}>
															<MaximizeIcon />
														</Button>
													</div>
												</div>
												<Code.Root code={segment.code} lang={segment.lang as SupportedLanguage} class="border-0 rounded-none bg-transparent text-xs" />
											</div>
										{/if}
									{/each}
								</div>
							{/if}
						</Chat.BubbleMessage>
					</Chat.Bubble>
				{/each}

				{#if loading}
					<Chat.Bubble variant="received">
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
