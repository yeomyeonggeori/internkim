<script lang="ts">
	import * as Marker from '$lib/components/ui/marker/index.js';

	let { label }: { label: string } = $props();
</script>

<Marker.Root role="status">
	<Marker.Content class="shimmer">{label}</Marker.Content>
</Marker.Root>
