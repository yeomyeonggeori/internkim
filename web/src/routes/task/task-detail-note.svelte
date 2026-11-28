<script lang="ts">
	import { Separator } from '$lib/components/ui/separator';

	type Props = {
		note: string | undefined;
		label: string;
	};

	let { note, label }: Props = $props();
</script>

{#if note?.trim()}
	<Separator />
	<div>
		<p class="text-xs text-muted-foreground">{label}</p>
		<p class="mt-1 whitespace-pre-wrap break-words text-sm leading-6">{note}</p>
	</div>
{/if}
