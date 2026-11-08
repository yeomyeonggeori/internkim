<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import ChannelAttachmentGroup from '$lib/components/channel/channel-attachment-group.svelte';
	import type { ChannelMessageAttachment } from '$lib/components/channel/channel-api';
	import { lightboxItemsOf } from '$lib/components/channel/channel-attachments';
	import ChannelLightbox, { type LightboxView } from '$lib/components/channel/channel-lightbox.svelte';
	import ChannelLinkCard from '$lib/components/channel/channel-link-card.svelte';
	import ChannelTextBubble from '$lib/components/channel/channel-text-bubble.svelte';
	import LiveWorkBubble from '$lib/components/channel/live-work-bubble.svelte';
	import SenderAvatar from '$lib/components/channel/sender-avatar.svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Card from '$lib/components/ui/card';
	import * as Message from '$lib/components/ui/message';
	import * as Tabs from '$lib/components/ui/tabs';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import type { ShownStep } from '$lib/messenger/agent-work';
	import { landingText } from './text';

	const text = createPageText(landingText);
	const agentPicture = '/agent-avatar.png';
	const personTurnMilliseconds = 900;
	const stepMilliseconds = 650;
	const replyMilliseconds = 700;
	const readingMilliseconds = 4200;
	const sampleFiles = { pdf: '/landing/sample-quote.pdf', xlsx: '/landing/sample-quote-items.xlsx' };

	type Frame = { shownTurns: number; workingSteps: number };

	let shownID = $state<string>(text.demo.scenarios[0].id);
	let frame = $state<Frame>(finishedFrameOf(0));
	let isAutoplaying = true;
	let prefersReducedMotion = false;
	let timers: ReturnType<typeof setTimeout>[] = [];
	let nextScenarioTimer: ReturnType<typeof setTimeout> | undefined;
	let lightbox = $state<LightboxView | null>(null);

	const shownIndex = $derived(indexOf(shownID));
	const scenario = $derived(text.demo.scenarios[shownIndex]);
	const workingTurn = $derived(frame.workingSteps > 0 ? scenario.turns[frame.shownTurns] : undefined);
	const workSteps = $derived(workingTurn ? stepsUpTo(workingTurn.steps, frame.workingSteps) : []);

	function indexOf(id: string): number {
		return Math.max(0, text.demo.scenarios.findIndex((item) => item.id === id));
	}

	function finishedFrameOf(index: number): Frame {
		return { shownTurns: text.demo.scenarios[index].turns.length, workingSteps: 0 };
	}

	function stepsUpTo(titles: readonly string[], count: number): ShownStep[] {
		return titles.slice(0, count).map((title, index) => ({ title, status: index === count - 1 ? 'in_progress' : 'completed' }));
	}

	function timelineOf(index: number): { frame: Frame; after: number }[] {
		const timeline = [{ frame: { shownTurns: 1, workingSteps: 0 }, after: 0 }];
		let elapsed = 0;
		text.demo.scenarios[index].turns.forEach((turn, turnIndex) => {
			if (turnIndex === 0) return;
			if (turn.from === 'person') {
				elapsed += personTurnMilliseconds;
				timeline.push({ frame: { shownTurns: turnIndex + 1, workingSteps: 0 }, after: elapsed });
				return;
			}
			turn.steps.forEach((_, stepIndex) => {
				elapsed += stepMilliseconds;
				timeline.push({ frame: { shownTurns: turnIndex, workingSteps: stepIndex + 1 }, after: elapsed });
			});
			elapsed += replyMilliseconds;
			timeline.push({ frame: { shownTurns: turnIndex + 1, workingSteps: 0 }, after: elapsed });
		});
		return timeline;
	}

	function fileAttachments(filename: string): ChannelMessageAttachment[] {
		if (!filename) return [];
		const source = filename.endsWith('.pdf') ? sampleFiles.pdf : sampleFiles.xlsx;
		return [{ kind: 'file', url: source, source, filename }];
	}

	function openFile(filename: string) {
		stopAutoplay();
		lightbox = { items: lightboxItemsOf(fileAttachments(filename)), index: 0 };
	}

	function clearTimers() {
		timers.forEach(clearTimeout);
		timers = [];
	}

	function scheduleNextScenario(index: number, after: number) {
		clearTimeout(nextScenarioTimer);
		if (!isAutoplaying) return;
		nextScenarioTimer = setTimeout(() => play((index + 1) % text.demo.scenarios.length), after);
	}

	function stopAutoplay() {
		isAutoplaying = false;
		clearTimeout(nextScenarioTimer);
	}

	function stopAutoplayOnceScrolledBack(event: Event) {
		if (event.currentTarget instanceof HTMLElement && event.currentTarget.scrollTop !== 0) stopAutoplay();
	}

	function play(index: number) {
		clearTimers();
		shownID = text.demo.scenarios[index].id;
		const timeline = timelineOf(index);
		frame = timeline[0].frame;
		timeline.forEach((step) => timers.push(setTimeout(() => (frame = step.frame), step.after)));
		scheduleNextScenario(index, timeline[timeline.length - 1].after + readingMilliseconds);
	}

	function choose(id: string) {
		stopAutoplay();
		if (prefersReducedMotion) {
			shownID = id;
			frame = finishedFrameOf(indexOf(id));
			return;
		}
		play(indexOf(id));
	}

	onMount(() => {
		prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
		isAutoplaying = !prefersReducedMotion;
		scheduleNextScenario(shownIndex, readingMilliseconds);
		return () => {
			clearTimers();
			clearTimeout(nextScenarioTimer);
		};
	});
</script>

{#snippet agentAvatar()}
	<SenderAvatar>
		<PersonAvatar name={text.demo.conversation} image={agentPicture} />
	</SenderAvatar>
{/snippet}

{#snippet files(filename: string)}
	{#if filename}
		<div class="relative w-fit max-w-[80%] self-start group-data-[align=end]/message:self-end">
			<ChannelAttachmentGroup attachments={fileAttachments(filename)} openSource={() => openFile(filename)} />
		</div>
	{/if}
{/snippet}

<Card.Root aria-label={text.demo.label}>
	<Card.Header>
		<div class="flex items-center gap-2.5">
			<PersonAvatar name={text.demo.conversation} image={agentPicture} />
			<Card.Title>{text.demo.conversation}</Card.Title>
		</div>
	</Card.Header>

	<Card.Content
		class="@container/conversation flex h-[27rem] flex-col-reverse overflow-y-auto pointer-coarse:overflow-hidden pointer-coarse:[mask-image:linear-gradient(to_bottom,transparent,black_2.5rem)]"
		onscroll={stopAutoplayOnceScrolledBack}
	>
		{#key shownID}
			<div class="flex flex-col gap-4">
				{#each scenario.turns.slice(0, frame.shownTurns) as turn, turnIndex (turnIndex)}
					<div in:fly={{ y: 12, duration: 250 }}>
						<Message.Root align={turn.from === 'person' ? 'end' : 'start'}>
							{#if turn.from === 'agent'}{@render agentAvatar()}{/if}
							<Message.Content>
								{#if turn.from === 'person'}{@render files(turn.file)}{/if}
								<ChannelTextBubble mine={turn.from === 'person'} source={turn.text} />
								{#if turn.from === 'agent'}{@render files(turn.file)}{/if}
								{#if turn.link.url}
									<ChannelLinkCard preview={{ ...turn.link, description: '', imageURL: '', video: null }} />
								{/if}
							</Message.Content>
						</Message.Root>
					</div>
				{/each}
				{#if workSteps.length > 0}
					<Message.Root align="start">
						{@render agentAvatar()}
						<Message.Content>
							<LiveWorkBubble steps={workSteps} label={text.demo.working} />
						</Message.Content>
					</Message.Root>
				{/if}
			</div>
		{/key}
	</Card.Content>

	<Card.Footer>
		<Tabs.Root value={shownID} onValueChange={choose} class="w-full">
			<Tabs.List class="w-full">
				{#each text.demo.scenarios as item (item.id)}
					<Tabs.Trigger value={item.id}>{item.tab}</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
	</Card.Footer>
</Card.Root>

<ChannelLightbox bind:view={lightbox} />
