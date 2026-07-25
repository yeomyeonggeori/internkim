<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as InputGroup from '$lib/components/ui/input-group/index.js';
	import * as Marker from '$lib/components/ui/marker/index.js';
	import * as Message from '$lib/components/ui/message/index.js';
	import * as MessageScroller from '$lib/components/ui/message-scroller/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { agentDmText } from '$lib/i18n/agent-dm-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import {
		fetchAgentConversation,
		sendAgentDirectMessage,
		type AgentDirectMessage
	} from './agent-dm-api';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import MessageCircleDashedIcon from '@lucide/svelte/icons/message-circle-dashed';
	import { onDestroy, onMount } from 'svelte';

	let { isActive = true }: { isActive?: boolean } = $props();

	const text = createPageText(agentDmText);
	const idleRefreshIntervalMs = 5000;
	const workingRefreshIntervalMs = 1500;

	let messages = $state<AgentDirectMessage[]>([]);
	let isAgentWorking = $state(false);
	let composerValue = $state('');
	let isSending = $state(false);
	let loadFailed = $state(false);
	let hasLoadedOnce = $state(false);
	let refreshTimer: ReturnType<typeof setTimeout> | undefined;

	async function loadConversation() {
		try {
			const conversation = await fetchAgentConversation();
			const latestIncoming = conversation.messages.at(-1);
			if (latestIncoming?.author === 'agent' && latestIncoming.id !== messages.at(-1)?.id) {
				isAgentWorking = false;
			}
			messages = conversation.messages;
			loadFailed = false;
		} catch {
			loadFailed = true;
		} finally {
			hasLoadedOnce = true;
		}
	}

	function scheduleRefresh() {
		clearTimeout(refreshTimer);
		if (!isActive) return;
		const delay = isAgentWorking ? workingRefreshIntervalMs : idleRefreshIntervalMs;
		refreshTimer = setTimeout(async () => {
			await loadConversation();
			scheduleRefresh();
		}, delay);
	}

	async function submitMessage(event: SubmitEvent) {
		event.preventDefault();
		const trimmedMessage = composerValue.trim();
		if (!trimmedMessage || isSending) return;
		isSending = true;
		composerValue = '';
		messages = [
			...messages,
			{
				id: `pending-${messages.length}`,
				author: 'me',
				text: trimmedMessage,
				sentAt: new Date().toISOString()
			}
		];
		try {
			await sendAgentDirectMessage(trimmedMessage);
			isAgentWorking = true;
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isSending = false;
			scheduleRefresh();
		}
	}

	async function answerChoice(optionLabel: string) {
		if (isSending) return;
		isSending = true;
		try {
			await sendAgentDirectMessage(optionLabel);
			isAgentWorking = true;
			await loadConversation();
		} catch {
			loadFailed = true;
		} finally {
			isSending = false;
			scheduleRefresh();
		}
	}

	function handleComposerKeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
		event.preventDefault();
		event.currentTarget?.dispatchEvent(new SubmitEvent('submit', { cancelable: true, bubbles: true }));
	}

	$effect(() => {
		if (isActive) scheduleRefresh();
		else clearTimeout(refreshTimer);
	});

	onMount(async () => {
		await loadConversation();
		scheduleRefresh();
	});

	onDestroy(() => clearTimeout(refreshTimer));
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="min-h-0 flex-1 overflow-hidden">
		{#if !hasLoadedOnce}
			<Empty.Root class="h-full">
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{text.title}</Empty.Title>
				</Empty.Header>
			</Empty.Root>
		{:else if loadFailed && messages.length === 0}
			<Empty.Root class="h-full">
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{text.unavailableTitle}</Empty.Title>
					<Empty.Description>{text.unavailableDescription}</Empty.Description>
				</Empty.Header>
				<Button variant="outline" size="sm" onclick={loadConversation}>{text.retry}</Button>
			</Empty.Root>
		{:else if messages.length === 0}
			<Empty.Root class="h-full">
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{text.emptyTitle}</Empty.Title>
					<Empty.Description>{text.emptyDescription}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<MessageScroller.Provider autoScroll>
				<MessageScroller.Root>
					<MessageScroller.Viewport>
						<MessageScroller.Content aria-busy={isAgentWorking} class="gap-4 p-4">
							{#each messages as message (message.id)}
								<MessageScroller.Item
									messageId={message.id}
									scrollAnchor={message.author === 'me'}
								>
									<Message.Root align={message.author === 'me' ? 'end' : 'start'}>
										<Message.Content>
											<Bubble.Root variant={message.author === 'me' ? 'default' : 'muted'}>
												<Bubble.Content>{message.text}</Bubble.Content>
											</Bubble.Root>
											{#if message.interaction}
												<div class="mt-2 flex flex-wrap gap-2">
													{#each message.interaction.options as option (option.key)}
														<Button
															size="sm"
															variant={option.key === message.interaction.recommendedOptionKey
																? 'default'
																: 'outline'}
															disabled={isSending}
															onclick={() => answerChoice(option.label)}
														>
															{option.label}
														</Button>
													{/each}
												</div>
											{/if}
										</Message.Content>
									</Message.Root>
								</MessageScroller.Item>
							{/each}
							{#if isAgentWorking}
								<Marker.Root role="status">
									<Marker.Content class="shimmer">{text.working}</Marker.Content>
								</Marker.Root>
							{/if}
						</MessageScroller.Content>
					</MessageScroller.Viewport>
					<MessageScroller.Button />
				</MessageScroller.Root>
			</MessageScroller.Provider>
		{/if}
	</div>
	<form onsubmit={submitMessage} class="border-t p-3">
		<InputGroup.Root>
			<InputGroup.Textarea
				bind:value={composerValue}
				placeholder={text.composerPlaceholder}
				aria-label={text.composerPlaceholder}
				rows={2}
				onkeydown={handleComposerKeydown}
			/>
			<InputGroup.Addon align="block-end" class="pt-1">
				<InputGroup.Button
					type="submit"
					variant="default"
					size="icon-sm"
					class="ms-auto"
					disabled={composerValue.trim().length === 0 || isSending}
				>
					<ArrowUpIcon />
					<span class="sr-only">{text.send}</span>
				</InputGroup.Button>
			</InputGroup.Addon>
		</InputGroup.Root>
	</form>
</div>
