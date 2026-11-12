<script lang="ts">
	import * as Bubble from '$lib/components/ui/bubble/index.js';
	import { bubbleReactionsVariants, bubbleVariants } from '$lib/components/ui/bubble/index.js';
	import * as Message from '$lib/components/ui/message/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import DesignFrame from './design-frame.svelte';
	import DesignSection from './design-section.svelte';
	import { designText } from './text';
	import { variantAxes, variantValues } from './workspace.svelte';

	let { pageID }: { pageID: string } = $props();

	const text = createPageText(designText);
</script>

<DesignSection {pageID} title={text.messagingSection}>
	<DesignFrame id="bubble" name="Bubble" source="ui/bubble" variants={[...variantAxes(bubbleVariants.variants), ...variantAxes(bubbleReactionsVariants.variants)]} class="w-96">
		{#snippet children()}
			{#each variantValues(bubbleVariants.variants.variant) as variant (variant)}
				<Bubble.Root {variant} align={variant === 'default' ? 'end' : 'start'}>
					<Bubble.Content>{variant}</Bubble.Content>
				</Bubble.Root>
			{/each}
			<Bubble.Group>
				<Bubble.Root variant="muted">
					<Bubble.Content>{text.sample.bubbleGroupOne}</Bubble.Content>
				</Bubble.Root>
				<Bubble.Root variant="muted" class="mb-4">
					<Bubble.Content>{text.sample.bubbleGroupTwo}</Bubble.Content>
					<Bubble.Reactions>👍 2</Bubble.Reactions>
				</Bubble.Root>
			</Bubble.Group>
		{/snippet}
	</DesignFrame>

	<DesignFrame id="message" name="Message" source="ui/message" class="w-96">
		{#snippet children()}
			<Message.Group>
				<Message.Root>
					<Message.Avatar><PersonAvatar name="박예시" seed="design-message" class="size-8" /></Message.Avatar>
					<Message.Content>
						<Message.Header>박예시</Message.Header>
						<Bubble.Root variant="muted"><Bubble.Content>{text.sample.bubbleGroupOne}</Bubble.Content></Bubble.Root>
						<Message.Footer>{text.sample.time}</Message.Footer>
					</Message.Content>
				</Message.Root>
				<Message.Root align="end">
					<Message.Content>
						<Bubble.Root align="end"><Bubble.Content>{text.sample.bubbleGroupTwo}</Bubble.Content></Bubble.Root>
					</Message.Content>
				</Message.Root>
			</Message.Group>
		{/snippet}
	</DesignFrame>
</DesignSection>
