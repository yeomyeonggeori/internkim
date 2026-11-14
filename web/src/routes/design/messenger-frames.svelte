<script lang="ts">
	import * as Empty from '$lib/components/ui/empty/index.js';
	import * as Marker from '$lib/components/ui/marker/index.js';
	import * as Message from '$lib/components/ui/message/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import LiveWorkBubble from '$lib/components/channel/live-work-bubble.svelte';
	import TypingBubble from '$lib/components/channel/typing-bubble.svelte';
	import AgentErrorBubble from '$lib/components/channel/agent-error-bubble.svelte';
	import ChoiceCard from '$lib/components/channel/choice-card.svelte';
	import { choiceCardOf } from '$lib/components/channel/choice-card';
	import type { ChannelInteraction } from '$lib/components/channel/channel-api';
	import ChannelTextBubble from '$lib/components/channel/channel-text-bubble.svelte';
	import ChannelLinkCard from '$lib/components/channel/channel-link-card.svelte';
	import FailedMessageActions from '$lib/components/channel/failed-message-actions.svelte';
	import ActivityMarker from '$lib/components/channel/activity-marker.svelte';
	import ChannelComposer from '$lib/components/channel/channel-composer.svelte';
	import ChannelLoadingSkeleton from '$lib/components/channel/channel-loading-skeleton.svelte';
	import JumpToLatestButton from '$lib/components/channel/jump-to-latest-button.svelte';
	import MessageCircleDashedIcon from '@lucide/svelte/icons/message-circle-dashed';
	import { channelText } from '$lib/i18n/channel-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { activityLabel, typingLabel, type Typist } from '$lib/messenger/typing-signal';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';

	type SamplePerson = { name: string; seed: string };

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
	const channel = createPageText(channelText);

	const agent: SamplePerson = $derived({ name: channel.title, seed: 'design-agent' });
	const sample: SamplePerson = { name: '이샘플', seed: 'design-sample' };
	const example: SamplePerson = { name: '박예시', seed: 'design-example' };
	const specimen: SamplePerson = { name: '최견본', seed: 'design-specimen' };

	const agentTypist: Typist = $derived({ name: agent.name, isAgent: true });
	const personTypist = (person: SamplePerson): Typist => ({ name: person.name, isAgent: false });

	const doNothing = () => {};

	const reader = 'design-reader';
	const approvalLabels = $derived({ approve: channel.approvalApprove, reject: channel.approvalReject });
	const question = $derived<ChannelInteraction>({
		interactionID: 'design-question',
		kind: 'choice',
		targetID: reader,
		options: [
			{ key: 'sales', label: text.sampleChoiceSales, isRecommended: true },
			{ key: 'engineering', label: text.sampleChoiceEngineering, isRecommended: false }
		]
	});
	const approval: ChannelInteraction = {
		interactionID: 'design-approval',
		kind: 'approval',
		targetID: reader,
		options: [
			{ key: 'approve', label: '', isRecommended: false },
			{ key: 'reject', label: '', isRecommended: false }
		]
	};
	const cardFor = (interaction: ChannelInteraction, readerID = reader) =>
		choiceCardOf(interaction, readerID, undefined, approvalLabels);
	const sendNothing = async () => {};
</script>

{#snippet avatar(person: SamplePerson, isOnline?: boolean)}
	<PersonAvatar
		name={person.name}
		seed={person.seed}
		{isOnline}
		presenceLabel={isOnline === undefined ? '' : isOnline ? channel.presenceOnline : channel.presenceOffline}
		class="size-8"
	/>
{/snippet}

{#snippet questionWith(body: string, card: ReturnType<typeof cardFor>)}
	<Message.Root>
		{@render avatar(agent)}
		<Message.Content>
			<ChannelTextBubble mine={false} source={body} />
			{#if card}
				<ChoiceCard {card} onChoose={doNothing} />
			{/if}
		</Message.Content>
	</Message.Root>
{/snippet}

{#snippet answer(body: string)}
	<Message.Root align="end">
		<Message.Content><ChannelTextBubble mine source={body} /></Message.Content>
	</Message.Root>
{/snippet}

{#snippet dock(label: string)}
	<div class="grid gap-1">
		<div class="flex h-6 items-center"><ActivityMarker {label} /></div>
	</div>
{/snippet}

<DesignSection {pageID} title={text.agentSection} description={text.agentSectionDescription}>
		<DesignFrame id="messenger-agent-working" source="channel/live-work-bubble.svelte" name={text.agentWorkingTitle} note={text.agentWorkingCondition}>
			{#snippet children()}
			<Message.Root align="end">
				<Message.Content><ChannelTextBubble mine source={text.sampleAsk} /></Message.Content>
			</Message.Root>
			<Message.Root>
				{@render avatar(agent)}
				<Message.Content><LiveWorkBubble steps={[]} label={channel.working} /></Message.Content>
			</Message.Root>
			{@render dock(activityLabel({ typists: [], isAgentWorking: true }, channel))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-steps" source="channel/live-work-bubble.svelte" name={text.agentStepsTitle} note={text.agentStepsCondition}>
			{#snippet children()}
			<Message.Root>
				{@render avatar(agent)}
				<Message.Content>
					<LiveWorkBubble
						label={channel.working}
						steps={[
							{ title: text.sampleStepRead, status: 'completed' },
							{ title: text.sampleStepRoom, status: 'failed' },
							{ title: text.sampleStepWrite, status: 'in_progress' }
						]}
					/>
				</Message.Content>
			</Message.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-typing" source="channel/typing-bubble.svelte" name={text.agentTypingTitle} note={text.agentTypingCondition}>
			{#snippet children()}
			<Message.Root>
				{@render avatar(agent)}
				<Message.Content><TypingBubble label={typingLabel([agentTypist], channel)} /></Message.Content>
			</Message.Root>
			{@render dock(typingLabel([agentTypist], channel))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-dock" source="channel/activity-marker.svelte" name={text.agentDockTitle} note={text.agentDockCondition}>
			{#snippet children()}
			{@render dock(activityLabel({ typists: [], isAgentWorking: true }, channel))}
			{@render dock(activityLabel({ typists: [agentTypist], isAgentWorking: false }, channel))}
			{@render dock(activityLabel({ typists: [personTypist(example)], isAgentWorking: true }, channel))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-error" source="channel/agent-error-bubble.svelte" name={text.agentErrorTitle} note={text.agentErrorCondition}>
			{#snippet children()}
			<Message.Root>
				{@render avatar(agent)}
				<Message.Content>
					<AgentErrorBubble text={text.sampleErrorNotice} />
				</Message.Content>
			</Message.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-choice" source="channel/choice-card.svelte" name={text.agentChoiceTitle} note={text.agentChoiceCondition}>
			{#snippet children()}
			{@render questionWith(text.sampleQuestion, cardFor(question))}
			{@render questionWith(text.sampleQuestion, cardFor(question, 'someone-else'))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-approval" source="channel/choice-card.svelte" name={text.agentApprovalTitle} note={text.agentApprovalCondition}>
			{#snippet children()}
			{@render questionWith(text.sampleApproval, cardFor(approval))}
			{@render questionWith(text.sampleApproval, cardFor(approval, 'someone-else'))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-choice-answered" source="channel/choice-card.svelte" name={text.agentChoiceAnsweredTitle} note={text.agentChoiceAnsweredCondition}>
			{#snippet children()}
			{@render questionWith(text.sampleApproval, cardFor({ ...approval, resolution: { key: 'approve' } }))}
			{@render answer(channel.approvalApprove)}
			{@render questionWith(text.sampleQuestion, cardFor({ ...question, resolution: {} }))}
			{@render answer(text.sampleOwnWordsAnswer)}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-agent-presence" source="person-avatar.svelte" name={text.agentPresenceTitle} note={text.agentPresenceCondition}>
			{#snippet children()}
			<div class="flex items-center gap-3">
				{@render avatar(agent, true)}
				<span class="text-sm font-medium">{agent.name}</span>
				<span class="text-muted-foreground text-xs">{channel.presenceOnline}</span>
			</div>
			{/snippet}
		</DesignFrame>
	</DesignSection>

<DesignSection {pageID} title={text.peopleSection} description={text.peopleSectionDescription}>
		<DesignFrame id="messenger-person-typing" source="channel/typing-bubble.svelte" name={text.personTypingTitle} note={text.personTypingCondition}>
			{#snippet children()}
			{#each [example, specimen, sample] as person (person.seed)}
				<Message.Root>
					{@render avatar(person)}
					<Message.Content><TypingBubble label={typingLabel([personTypist(person)], channel)} /></Message.Content>
				</Message.Root>
			{/each}
			{@render dock(typingLabel([example, specimen, sample].map(personTypist), channel))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-person-dock" source="channel/activity-marker.svelte" name={text.personDockTitle} note={text.personDockCondition}>
			{#snippet children()}
			{@render dock(typingLabel([personTypist(example)], channel))}
			{@render dock(typingLabel([personTypist(example), personTypist(specimen)], channel))}
			{@render dock(typingLabel([example, specimen, sample].map(personTypist), channel))}
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-person-presence" source="person-avatar.svelte" name={text.personPresenceTitle} note={text.personPresenceCondition}>
			{#snippet children()}
			<div class="grid gap-3">
				<div class="flex items-center gap-3">
					{@render avatar(sample, true)}
					<span class="text-sm font-medium">{sample.name}</span>
					<span class="text-muted-foreground text-xs">{channel.presenceOnline}</span>
				</div>
				<div class="flex items-center gap-3">
					{@render avatar(example, false)}
					<span class="text-sm font-medium">{example.name}</span>
					<span class="text-muted-foreground text-xs">{channel.presenceOffline}</span>
				</div>
			</div>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-messages" source="channel/channel-text-bubble.svelte" name={text.messagesTitle} note={text.messagesCondition}>
			{#snippet children()}
			<Message.Root>
				{@render avatar(example)}
				<Message.Content>
					<Message.Header class="text-muted-foreground -mb-2 px-0 text-xs font-medium">{example.name}</Message.Header>
					<ChannelTextBubble mine={false} source={text.sampleTheirs} />
				</Message.Content>
			</Message.Root>
			<Message.Root align="end">
				<Message.Content><ChannelTextBubble mine source={text.sampleMine} /></Message.Content>
			</Message.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-failed" source="channel/failed-message-actions.svelte" name={text.failedTitle} note={text.failedCondition}>
			{#snippet children()}
			<Message.Root align="end">
				<Message.Content>
					<ChannelTextBubble mine source={text.sampleQuote} />
					<FailedMessageActions onRetry={doNothing} onDiscard={doNothing} />
				</Message.Content>
			</Message.Root>
			<Message.Root align="end">
				<Message.Content>
					<FailedMessageActions isTooLarge onRetry={doNothing} onDiscard={doNothing} />
				</Message.Content>
			</Message.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-link" source="channel/channel-link-card.svelte" name={text.linkTitle} note={text.linkCondition}>
			{#snippet children()}
			<Message.Root>
				{@render avatar(specimen)}
				<Message.Content>
					<ChannelTextBubble mine={false} source={text.sampleLinkMessage} />
					<ChannelLinkCard
						preview={{
							url: 'https://example.com/news',
							title: text.sampleLinkTitle,
							description: text.sampleLinkDescription,
							siteName: text.sampleLinkSite,
							imageURL: '',
							video: null
						}}
					/>
				</Message.Content>
			</Message.Root>
			{/snippet}
		</DesignFrame>
	</DesignSection>

<DesignSection {pageID} title={text.sharedSection} description={text.sharedSectionDescription}>
		<DesignFrame id="messenger-date" source="ui/marker" name={text.dateTitle} note={text.dateCondition}>
			{#snippet children()}
			<Marker.Root variant="separator"><Marker.Content>{text.sampleDate}</Marker.Content></Marker.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-empty" source="ui/empty" name={text.emptyTitle} note={text.emptyCondition}>
			{#snippet children()}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon"><MessageCircleDashedIcon /></Empty.Media>
					<Empty.Title>{channel.emptyTitle}</Empty.Title>
					<Empty.Description>{channel.emptyDescription}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-loading" source="channel/channel-loading-skeleton.svelte" name={text.loadingTitle} note={text.loadingCondition}>
			{#snippet children()}
			<div class="h-56 overflow-hidden"><ChannelLoadingSkeleton label={channel.loadingMessages} /></div>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-jump" source="channel/jump-to-latest-button.svelte" name={text.jumpTitle} note={text.jumpCondition}>
			{#snippet children()}
			<div class="flex justify-center">
				<JumpToLatestButton isVisible label={channel.jumpToLatest} onclick={doNothing} />
			</div>
			{/snippet}
		</DesignFrame>

		<DesignFrame id="messenger-composer" source="channel/channel-composer.svelte" name={text.composerTitle} note={text.composerCondition}>
			{#snippet children()}
			<ChannelComposer name="design-composer" placeholder={channel.composerPlaceholder} participants={[]} isGroup={false} onSend={sendNothing} />
			<ChannelComposer
				name="design-composer-disabled"
				placeholder={channel.composerDisabledPlaceholder}
				participants={[]}
				isGroup={false}
				disabled
				onSend={sendNothing}
			/>
			{/snippet}
		</DesignFrame>
	</DesignSection>
