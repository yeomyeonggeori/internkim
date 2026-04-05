<script lang="ts">
	import * as Chat from '$lib/components/ui/chat';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import SendIcon from '@lucide/svelte/icons/send';
	import BotIcon from '@lucide/svelte/icons/bot';
	import SettingsIcon from '@lucide/svelte/icons/settings';
	import CheckIcon from '@lucide/svelte/icons/check';
	import LoaderIcon from '@lucide/svelte/icons/loader';
	import { marked } from 'marked';
	import DOMPurify from 'dompurify';

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

	let currentModel = $state('');
	let modelInput = $state('');
	let showSettings = $state(false);
	let modelSaving = $state(false);

	function renderMarkdown(text: string): string {
		const raw = marked.parse(text, { async: false }) as string;
		return DOMPurify.sanitize(raw);
	}

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
	<div class="bg-background flex items-center justify-between border-b p-2">
		<div class="flex items-center gap-2 pl-1">
			<Avatar.Root class="size-8">
				<Avatar.Fallback class="bg-primary text-primary-foreground text-xs">
					<BotIcon class="size-4" />
				</Avatar.Fallback>
			</Avatar.Root>
			<div class="flex flex-col">
				<span class="text-sm font-medium">Quick Claw</span>
				{#if currentModel}
					<span class="text-muted-foreground text-xs">{currentModel}</span>
				{/if}
			</div>
		</div>
		<div class="flex items-center">
			{#if showSettings}
				<div class="flex items-center gap-1">
					<input
						bind:value={modelInput}
						onkeydown={onModelKeydown}
						placeholder="model ID (e.g. google/gemini-3.1-flash-lite-preview)"
						class="border-input bg-background w-64 rounded-lg border px-2.5 py-1.5 text-xs outline-none"
					/>
					<Button
						variant="ghost"
						size="icon-sm"
						class="rounded-full"
						onclick={saveModel}
						disabled={modelSaving}
					>
						{#if modelSaving}
							<LoaderIcon class="animate-spin" />
						{:else}
							<CheckIcon />
						{/if}
					</Button>
				</div>
			{:else}
				<Button
					variant="ghost"
					size="icon"
					class="rounded-full"
					onclick={() => {
						showSettings = true;
						modelInput = currentModel;
					}}
				>
					<SettingsIcon />
				</Button>
			{/if}
		</div>
	</div>

	<Chat.List class="flex-1">
		{#if messages.length === 0}
			<div class="text-muted-foreground flex h-full items-center justify-center">
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
					{#if msg.role === 'assistant'}
						<div class="markdown max-w-none text-sm">
							{@html renderMarkdown(msg.content)}
						</div>
					{:else}
						{msg.content}
					{/if}
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

	<form
		onsubmit={(e) => {
			e.preventDefault();
			send();
		}}
		class="flex items-center gap-2 border-t p-2"
	>
		<input
			bind:value={input}
			{onkeydown}
			placeholder="Type a message..."
			class="border-input bg-background flex-1 rounded-full border px-4 py-2 text-sm outline-none"
		/>
		<Button
			type="submit"
			variant="default"
			size="icon"
			class="shrink-0 rounded-full"
			disabled={loading || !input.trim()}
		>
			<SendIcon />
		</Button>
	</form>
</div>


