<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { cn } from '$lib/utils';
	import type { ActivityPerson } from './activity';

	let { person, seed, class: className }: { person: ActivityPerson | undefined; seed: string; class?: string } = $props();
</script>

<PersonAvatar
	name={person?.name ?? ''}
	{seed}
	image={person?.image ?? ''}
	class={cn('[&_img]:scale-125 [&_img]:blur-[1.5px]', className)}
/>
