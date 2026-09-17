<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		finishHandoff,
		sendHandoffInputs,
		watchHandoff,
		type FinishingOutcome,
		type HandoffWatch
	} from '$lib/browser-handoff/handoff-api';
	import type { HandoffEvent, HandoffOutcome } from '$lib/browser-handoff/handoff-event';
	import type { HandoffInput, ScreenFrame, Viewport } from '$lib/browser-handoff/handoff-input';
	import { HandoffInputQueue } from '$lib/browser-handoff/handoff-input-queue';
	import { Button } from '$lib/components/ui/button';
	import * as Empty from '$lib/components/ui/empty';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { onHandoffEvent } from '$lib/host-bridge';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ArrowLeftIcon from '@lucide/svelte/icons/arrow-left';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import CircleXIcon from '@lucide/svelte/icons/circle-x';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import GlobeIcon from '@lucide/svelte/icons/globe';
	import KeyboardIcon from '@lucide/svelte/icons/keyboard';
	import MonitorOffIcon from '@lucide/svelte/icons/monitor-off';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import ShieldAlertIcon from '@lucide/svelte/icons/shield-alert';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import WifiOffIcon from '@lucide/svelte/icons/wifi-off';
	import XIcon from '@lucide/svelte/icons/x';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import HandoffScreen from '../handoff-screen.svelte';
	import { handoffText } from '../text';

	type Phase =
		| { name: 'connecting' }
		| { name: 'watching'; watch: HandoffWatch }
		| { name: 'missing' }
		| { name: 'refused' }
		| { name: 'unreachable' }
		| { name: 'failed'; reason: string }
		| { name: 'ended'; outcome: HandoffOutcome };

	const text = createPageText(handoffText);
	const watchRenewalMilliseconds = 15_000;
	const resizeSettleMilliseconds = 250;
	const troubleShownMilliseconds = 8_000;
	const inputFailureToastID = 'browser-handoff-input';
	const handoffID = page.params.id ?? '';
	const inputs = new HandoffInputQueue((batch) => sendHandoffInputs(handoffID, batch), reportInputFailure);

	let phase = $state<Phase>({ name: 'connecting' });
	let frame = $state<ScreenFrame | null>(null);
	let pageAddress = $state('');
	let trouble = $state('');
	let troubleTimer: ReturnType<typeof setTimeout> | undefined;
	let isFinishing = $state(false);
	let screen = $state<HandoffScreen>();
	let wantedViewport: Viewport | null = null;
	let resizeTimer: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		const stopListening = onHandoffEvent(receive);
		void keepWatching();
		const renewal = setInterval(() => {
			if (phase.name === 'watching') void keepWatching();
		}, watchRenewalMilliseconds);
		return () => {
			stopListening();
			clearInterval(renewal);
			clearTimeout(resizeTimer);
			clearTimeout(troubleTimer);
		};
	});

	async function keepWatching(): Promise<void> {
		try {
			const answer = await watchHandoff(handoffID, wantedViewport);
			if (phase.name === 'ended') return;
			if (answer.state !== 'failed') {
				phase = answer.state === 'watching' ? { name: 'watching', watch: answer.watch } : { name: answer.state };
				return;
			}
			console.warn('the device could not show the browser handoff', answer.reason);
			if (phase.name !== 'watching') phase = { name: 'failed', reason: answer.reason };
		} catch (failure) {
			console.warn('the browser handoff could not be watched', failure);
			if (phase.name !== 'watching') phase = { name: 'unreachable' };
		}
	}

	function reconnect(): void {
		phase = { name: 'connecting' };
		void keepWatching();
	}

	function followScreenSize(area: Viewport): void {
		wantedViewport = area;
		clearTimeout(resizeTimer);
		resizeTimer = setTimeout(() => void keepWatching(), resizeSettleMilliseconds);
	}

	function receive(event: HandoffEvent): void {
		if (event.handoffID !== handoffID) return;
		if (event.kind === 'ended') {
			phase = { name: 'ended', outcome: event.outcome };
			return;
		}
		if (event.kind === 'trouble') {
			showTrouble(event.reason);
			return;
		}
		frame = { image: `data:image/jpeg;base64,${event.image}`, width: event.width, height: event.height, fields: event.fields };
		if (event.url) pageAddress = shownAddressOf(event.url);
	}

	function shownAddressOf(url: string): string {
		if (!URL.canParse(url)) return url;
		const address = new URL(url);
		return `${address.host}${address.pathname === '/' ? '' : address.pathname}`;
	}

	function send(input: HandoffInput): void {
		if (phase.name !== 'watching') return;
		inputs.push(input);
	}

	function showTrouble(reason: string): void {
		trouble = reason;
		clearTimeout(troubleTimer);
		troubleTimer = setTimeout(() => (trouble = ''), troubleShownMilliseconds);
	}

	function reportInputFailure(failure: unknown): void {
		console.warn('the browser handoff did not take the input', failure);
		const reason = failure instanceof Error ? failure.message : String(failure);
		toast.error(text.inputFailed, { id: inputFailureToastID, description: reason });
	}

	async function finish(outcome: FinishingOutcome): Promise<void> {
		isFinishing = true;
		try {
			await finishHandoff(handoffID, outcome);
			phase = { name: 'ended', outcome };
		} catch (failure) {
			console.warn('the browser handoff could not be finished', failure);
			toast.error(text.finishFailed);
		} finally {
			isFinishing = false;
		}
	}

	function leave(): void {
		if (history.length > 1) history.back();
		else void goto('/');
	}

	function expiryTimeOf(watch: HandoffWatch): string {
		const time = new Date(watch.expiresAt).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
		return text.expiresAt.replace('{time}', time);
	}

	function endedTitleOf(outcome: HandoffOutcome): string {
		if (outcome === 'completed') return text.endedCompletedTitle;
		if (outcome === 'abandoned') return text.endedAbandonedTitle;
		return text.endedExpiredTitle;
	}

	function endedDescriptionOf(outcome: HandoffOutcome): string {
		if (outcome === 'completed') return text.endedCompletedDescription;
		if (outcome === 'abandoned') return text.endedAbandonedDescription;
		return text.endedExpiredDescription;
	}
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

{#if phase.name === 'watching'}
	<main class="flex min-h-0 w-full flex-1 flex-col bg-background" aria-label={text.title}>
		<header class="flex h-12 shrink-0 items-center gap-1 border-b px-2">
			<Button variant="ghost" size="icon-sm" aria-label={text.back} onclick={() => send({ type: 'history', direction: 'back' })}>
				<ArrowLeftIcon />
			</Button>
			<Button variant="ghost" size="icon-sm" aria-label={text.reload} onclick={() => send({ type: 'reload' })}>
				<RotateCwIcon />
			</Button>
			<div class="flex min-w-0 flex-1 flex-col px-1">
				<p class="truncate text-sm">{phase.watch.message || text.description}</p>
				<p class="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
					<ClockIcon class="size-3 shrink-0" />
					<span class="shrink-0">{expiryTimeOf(phase.watch)}</span>
					{#if trouble}
						<TriangleAlertIcon class="ml-1.5 size-3 shrink-0 text-destructive" />
						<span class="truncate text-destructive" title={trouble}>{text.slowBrowser}</span>
					{:else if pageAddress}
						<GlobeIcon class="ml-1.5 size-3 shrink-0" />
						<span class="truncate" title={pageAddress}>{pageAddress}</span>
					{/if}
				</p>
			</div>
			<Button variant="ghost" size="icon-sm" aria-label={text.keyboard} onclick={() => screen?.focusKeyboard()}>
				<KeyboardIcon />
			</Button>
			<Button variant="ghost" size="sm" disabled={isFinishing} onclick={() => finish('abandoned')}>
				<XIcon data-icon="inline-start" />
				{text.abandon}
			</Button>
			<Button size="sm" disabled={isFinishing} onclick={() => finish('completed')}>
				<CheckIcon data-icon="inline-start" />
				{text.complete}
			</Button>
		</header>
		<div class="min-h-0 flex-1">
			<HandoffScreen
				bind:this={screen}
				{frame}
				label={text.title}
				waitingText={text.waitingForScreen}
				onInput={send}
				onResize={followScreenSize}
			/>
		</div>
	</main>
{:else if phase.name === 'connecting'}
	<main class="flex min-h-0 w-full flex-1 flex-col" aria-label={text.connecting}>
		<div class="flex h-12 shrink-0 items-center border-b px-2"><Skeleton class="h-8 w-full" /></div>
		<Skeleton class="min-h-0 w-full flex-1 rounded-none" />
	</main>
{:else}
	<main class="flex w-full flex-1 items-center justify-center px-4 py-4">
		<Empty.Root>
			<Empty.Header>
				<Empty.Media variant="icon">
					{#if phase.name === 'ended'}
						{#if phase.outcome === 'completed'}<CircleCheckIcon />{:else}<CircleXIcon />{/if}
					{:else if phase.name === 'refused'}
						<ShieldAlertIcon />
					{:else if phase.name === 'unreachable'}
						<WifiOffIcon />
					{:else}
						<MonitorOffIcon />
					{/if}
				</Empty.Media>
				{#if phase.name === 'ended'}
					<Empty.Title>{endedTitleOf(phase.outcome)}</Empty.Title>
					<Empty.Description>{endedDescriptionOf(phase.outcome)}</Empty.Description>
				{:else if phase.name === 'refused'}
					<Empty.Title>{text.refusedTitle}</Empty.Title>
					<Empty.Description>{text.refusedDescription}</Empty.Description>
				{:else if phase.name === 'unreachable'}
					<Empty.Title>{text.unreachableTitle}</Empty.Title>
					<Empty.Description>{text.unreachableDescription}</Empty.Description>
				{:else if phase.name === 'failed'}
					<Empty.Title>{text.failedTitle}</Empty.Title>
					<Empty.Description>{text.failedDescription}</Empty.Description>
					<p class="font-mono text-xs break-words text-muted-foreground">{phase.reason}</p>
				{:else}
					<Empty.Title>{text.missingTitle}</Empty.Title>
					<Empty.Description>{text.missingDescription}</Empty.Description>
				{/if}
			</Empty.Header>
			<Empty.Content>
				<div class="flex gap-2">
					<Button variant="outline" size="sm" onclick={leave}>
						<ArrowLeftIcon data-icon="inline-start" />
						{text.leave}
					</Button>
					{#if phase.name === 'unreachable' || phase.name === 'failed'}
						<Button size="sm" onclick={reconnect}>
							<RotateCwIcon data-icon="inline-start" />
							{text.retry}
						</Button>
					{/if}
				</div>
			</Empty.Content>
		</Empty.Root>
	</main>
{/if}
