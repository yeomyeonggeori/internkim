<script lang="ts">
	import { CopyButton } from '$lib/components/ui/copy-button';

	let { document }: { document: string } = $props();
</script>

<div class="relative">
	<pre class="max-h-96 overflow-auto rounded-lg border bg-muted/30 py-2 pr-10 pl-3 text-xs leading-relaxed whitespace-pre-wrap">{document}</pre>
	<CopyButton text={document} size="icon-sm" class="absolute top-1 right-1" />
</div>
