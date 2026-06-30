<script lang="ts">
	type Props = {
		value: string;
		isMuted?: boolean;
		isTruncated?: boolean;
	};

	let { value, isMuted = false, isTruncated = false }: Props = $props();
</script>

<span class={[isMuted ? 'text-muted-foreground' : '', isTruncated ? 'block max-w-[26rem] truncate' : '']}>{value}</span>
