<script lang="ts">
	import * as Chat from '$lib/components/ui/chat';
	import SendIcon from '@lucide/svelte/icons/send';
	import BotIcon from '@lucide/svelte/icons/bot';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import CheckIcon from '@lucide/svelte/icons/check';
	import LoaderIcon from '@lucide/svelte/icons/loader';

	type Message = {
		role: 'user' | 'assistant';
		content: string;
	};

	const apiBase = () => (location.port === '5173' ? 'http://192.168.0.141:8090' : '');

	let messages = $state<Message[]>([]);
	let input = $state('');
	let loading = $state(false);
	let ws = $state<WebSocket | null>(null);
	let sessionId = $state('chat-' + crypto.randomUUID().slice(0, 8));
	let pendingResolve: ((value: string) => void) | null = null;
	let currentContent = $state('');

	// Model settings
	let currentModel = $state('');
	let modelInput = $state('');
	let showSettings = $state(false);
	let modelSaving = $state(false);

	async function loadModel() {
		try {
			const res = await fetch(`${apiBase()}/pico/model`);
			const data = await res.json();
			currentModel = data.model || '';
			modelInput = currentModel;
		} catch {
			/* ignore */
		}
	}

	async function saveModel() {
		if (!modelInput.trim() || modelInput === currentModel) {
			showSettings = false;
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
				// Reconnect WS since picoclaw restarted
				if (ws) {
					ws.close();
					ws = null;
				}
			}
		} catch {
			/* ignore */
		} finally {
			modelSaving = false;
			showSettings = false;
		}
	}

	$effect(() => {
		loadModel();
	});

	async function getToken(): Promise<{ token: string; ws_url: string }> {
		const res = await fetch(`${apiBase()}/pico/token`);
		return res.json();
	}

	async function connectWs() {
		const { token } = await getToken();
		const host = location.port === '5173' ? '192.168.0.141:18790' : location.host;
		const proto = location.protocol === 'https:' ? 'wss' : 'ws';
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

	async function send() {
		const text = input.trim();
		if (!text || loading) return;

		messages.push({ role: 'user', content: text });
		input = '';
		loading = true;
		currentContent = '';

		try {
			if (!ws || ws.readyState !== WebSocket.OPEN) {
				ws = await connectWs();
			}

			const content = await new Promise<string>((resolve) => {
				pendingResolve = resolve;
				ws!.send(
					JSON.stringify({
						type: 'message.send',
						id: crypto.randomUUID(),
						payload: { content: text }
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

	function onModelKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			saveModel();
		}
		if (e.key === 'Escape') {
			showSettings = false;
			modelInput = currentModel;
		}
	}
</script>

<svelte:head>
	<title>Quick Claw</title>
</svelte:head>

<div class="flex h-svh flex-col">
	<header class="border-b px-4 py-3">
		<div class="mx-auto flex max-w-2xl items-center gap-2">
			<BotIcon class="size-5 text-primary" />
			<h1 class="text-lg font-semibold">Quick Claw</h1>
			<span class="flex-1"></span>
			{#if showSettings}
				<div class="flex items-center gap-1">
					<input
						bind:value={modelInput}
						onkeydown={onModelKeydown}
						placeholder="model ID"
						class="border-input bg-background w-56 rounded border px-2 py-1 text-xs outline-none"
					/>
					<button
						onclick={saveModel}
						disabled={modelSaving}
						class="text-muted-foreground hover:text-foreground inline-flex size-7 items-center justify-center rounded"
					>
						{#if modelSaving}
							<LoaderIcon class="size-3.5 animate-spin" />
						{:else}
							<CheckIcon class="size-3.5" />
						{/if}
					</button>
				</div>
			{:else}
				<button
					onclick={() => {
						showSettings = true;
						modelInput = currentModel;
					}}
					class="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs"
				>
					{#if currentModel}
						<span class="max-w-40 truncate">{currentModel}</span>
					{/if}
					<SettingsIcon class="size-4" />
				</button>
			{/if}
		</div>
	</header>

	<div class="flex-1 overflow-hidden">
		<div class="mx-auto h-full max-w-2xl">
			<Chat.List class="h-full">
				{#if messages.length === 0}
					<div class="flex h-full items-center justify-center text-muted-foreground">
						<p>Send a message to start chatting.</p>
					</div>
				{/if}

				{#each messages as msg}
					<Chat.Bubble variant={msg.role === 'user' ? 'sent' : 'received'}>
						{#if msg.role === 'assistant'}
							<Chat.BubbleAvatar>
								<Chat.BubbleAvatarFallback>
									<BotIcon class="size-4" />
								</Chat.BubbleAvatarFallback>
							</Chat.BubbleAvatar>
						{/if}
						<Chat.BubbleMessage>
							{msg.content}
						</Chat.BubbleMessage>
					</Chat.Bubble>
				{/each}

				{#if loading}
					<Chat.Bubble variant="received">
						<Chat.BubbleAvatar>
							<Chat.BubbleAvatarFallback>
								<BotIcon class="size-4" />
							</Chat.BubbleAvatarFallback>
						</Chat.BubbleAvatar>
						<Chat.BubbleMessage typing />
					</Chat.Bubble>
				{/if}
			</Chat.List>
		</div>
	</div>

	<div class="border-t px-4 py-3">
		<div class="mx-auto flex max-w-2xl gap-2">
			<textarea
				bind:value={input}
				{onkeydown}
				placeholder="Type a message..."
				rows={1}
				class="border-input bg-background ring-ring/10 flex-1 resize-none rounded-lg border px-3 py-2 text-sm outline-none focus:ring-2"
			></textarea>
			<button
				onclick={send}
				disabled={loading || !input.trim()}
				class="bg-primary text-primary-foreground hover:bg-primary/90 inline-flex size-10 shrink-0 items-center justify-center rounded-lg disabled:opacity-50"
			>
				<SendIcon class="size-4" />
			</button>
		</div>
	</div>
</div>
