<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { setContext } from 'svelte';
	import ChannelCode from './channel-code.svelte';
	import ChannelMentionText from './channel-mention-text.svelte';
	import { mentionLabelsContext, type MentionLabel } from '$lib/messenger/mention-text';
	import { markdownKeepingBlankLines } from '$lib/messenger/message-blank-lines';

	let { source, mentionLabels = [] }: { source: string; mentionLabels?: MentionLabel[] } = $props();

	setContext(mentionLabelsContext, () => mentionLabels);

	const shown = $derived(markdownKeepingBlankLines(source));
</script>

<SvelteMarkdown
	source={shown}
	options={{ breaks: true, gfm: true }}
	renderers={{ code: ChannelCode, rawtext: ChannelMentionText }}
/>
