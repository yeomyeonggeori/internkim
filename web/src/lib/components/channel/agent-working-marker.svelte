<script lang="ts">
	import * as Marker from '$lib/components/ui/marker/index.js';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import type { ChannelParticipant } from './channel-api';

	let {
		agent,
		label
	}: {
		agent: ChannelParticipant;
		label: string;
	} = $props();
</script>

<Marker.Root role="status">
	<PersonAvatar
		class="size-6"
		name={agent.name}
		email={agent.email ?? ''}
		seed={agent.email || agent.id || agent.name}
		image={agent.avatarURL ?? ''}
		memberID={agent.memberID ?? ''}
		externalID={agent.externalID ?? ''}
	/>
	<Marker.Content class="shimmer">{label}</Marker.Content>
</Marker.Root>
