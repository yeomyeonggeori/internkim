<script lang="ts">
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { setContext } from 'svelte';
	import ChannelCode from './channel-code.svelte';
	import ChannelMentionText from './channel-mention-text.svelte';
	import { mentionLabelsContext } from '$lib/messenger/mention-text';

	let { source, mentionLabels = [] }: { source: string; mentionLabels?: string[] } = $props();

	setContext(mentionLabelsContext, () => mentionLabels);
</script>

<SvelteMarkdown
	{source}
	options={{ breaks: true, gfm: true }}
	renderers={{ code: ChannelCode, rawtext: ChannelMentionText }}
/>
