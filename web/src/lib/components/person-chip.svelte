<script lang="ts">
	import type { Snippet } from 'svelte';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { cn } from '$lib/utils';

	type Props = {
		name: string;
		email?: string;
		seed?: string;
		image?: string;
		memberID?: string;
		class?: string;
		children?: Snippet;
	};

	let { name, email = '', seed = '', image = '', memberID = '', class: className, children }: Props = $props();
</script>

<Badge variant="outline" class={cn('max-w-full gap-1.5 rounded-full pl-0.5', className)}>
	<PersonAvatar {name} {email} {seed} {image} {memberID} class="size-4 shrink-0" />
	<span class="min-w-0 truncate">{name}</span>
	{@render children?.()}
</Badge>
