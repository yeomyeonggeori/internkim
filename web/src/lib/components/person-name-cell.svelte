<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { personProfileImagePath } from '$lib/person-profile-image';
	import { cn } from '$lib/utils';

	type Props = {
		name: string;
		personID?: string;
		email?: string;
		fallback?: string;
		class?: string;
	};

	let { name, personID = '', email = '', fallback = '—', class: className }: Props = $props();
</script>

{#if name || personID}
	<div class={cn('flex min-w-0 items-center gap-2 font-medium', className)}>
		<PersonAvatar name={name || personID} {email} seed={personID || name} image={personProfileImagePath(personID)} class="size-6" />
		<span class="truncate">{name || personID}</span>
	</div>
{:else}
	<span class="text-muted-foreground">{fallback}</span>
{/if}
