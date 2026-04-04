<script lang="ts">
	import * as Chat from '$lib/components/ui/chat';
	import SendIcon from '@lucide/svelte/icons/send';
	import BotIcon from '@lucide/svelte/icons/bot';

	type Message = {
		role: 'user' | 'assistant';
		content: string;
	};

	let messages = $state<Message[]>([]);
	let input = $state('');
	let loading = $state(false);

	async function send() {
		const text = input.trim();
		if (!text || loading) return;

		messages.push({ role: 'user', content: text });
		input = '';
		loading = true;

		try {
			const res = await fetch('/v1/chat/completions', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					model: 'qwen-free',
					messages: messages.map((m) => ({ role: m.role, content: m.content })),
					stream: false
				})
			});

			if (!res.ok) {
				messages.push({ role: 'assistant', content: `Error: ${res.status} ${res.statusText}` });
				return;
			}

			const data = (await res.json()) as { choices?: { message?: { content?: string } }[] };
			const reply = data.choices?.[0]?.message?.content ?? 'No response';
			messages.push({ role: 'assistant', content: reply });
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
	<title>Quick Claw</title>
</svelte:head>

<div class="flex h-svh flex-col">
	<header class="border-b px-4 py-3">
		<div class="mx-auto flex max-w-2xl items-center gap-2">
			<BotIcon class="size-5 text-primary" />
			<h1 class="text-lg font-semibold">Quick Claw</h1>
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
