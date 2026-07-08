<script lang="ts">
	import MapPinIcon from '@lucide/svelte/icons/map-pin';

	type Props = {
		name: string;
		class?: string;
	};

	let { name, class: className = '' }: Props = $props();
</script>

<span class={`inline-flex min-w-0 items-center gap-1 text-xs text-muted-foreground ${className}`}>
	<MapPinIcon class="size-3 shrink-0" />
	<span class="min-w-0 truncate">{name}</span>
</span>
