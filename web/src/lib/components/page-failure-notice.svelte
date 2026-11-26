<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import HouseIcon from '@lucide/svelte/icons/house';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { homePath } from '$lib/home-path';

	let { detail }: { detail?: string } = $props();

	const isKorean = $derived(currentLocale.value === 'ko');

	function reload(): void {
		location.reload();
	}
</script>

<div role="alert" class="flex min-h-0 flex-1 items-center justify-center p-6">
	<Card.Root class="w-full max-w-md">
		<Card.Header>
			<Card.Title>{isKorean ? '문제가 발생했어요' : 'Something went wrong'}</Card.Title>
			{#if detail}
				<Card.Description>{detail}</Card.Description>
			{/if}
		</Card.Header>
		<Card.Content class="text-sm text-muted-foreground">
			{isKorean
				? '잠시 후 다시 시도하거나, 왼쪽 메뉴에서 다른 화면으로 이동할 수 있어요.'
				: 'Try again in a moment, or use the left menu to go to another screen.'}
		</Card.Content>
		<Card.Footer class="gap-2">
			<Button onclick={reload}>
				<RefreshCwIcon />
				{isKorean ? '다시 시도' : 'Retry'}
			</Button>
			<Button variant="outline" href={homePath}>
				<HouseIcon />
				{isKorean ? '홈으로' : 'Home'}
			</Button>
		</Card.Footer>
	</Card.Root>
</div>
