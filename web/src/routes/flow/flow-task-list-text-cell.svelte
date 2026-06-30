<!-- 업무 목록 테이블의 짧은 텍스트 셀을 렌더링한다. -->
<script lang="ts">
	type Props = {
		value: string;
		isMuted?: boolean;
		isTruncated?: boolean;
	};

	let { value, isMuted = false, isTruncated = false }: Props = $props();
</script>

<span class={[isMuted ? 'text-muted-foreground' : '', isTruncated ? 'block max-w-[26rem] truncate' : '']}>{value}</span>
