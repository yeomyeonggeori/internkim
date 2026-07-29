<script lang="ts">
	import { buzzIdentity } from '$lib/stores/buzz-identity.svelte';
	import { buzzPublicKeyOf } from '$lib/buzz-relay-client';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { npubEncode, nsecEncode } from 'nostr-tools/nip19';
	import QrCode from 'svelte-qrcode';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import SmartphoneIcon from '@lucide/svelte/icons/smartphone';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const text = createPageText({
		ko: {
			title: 'Buzz 앱 연결',
			description: '모바일·데스크톱 Buzz 앱에서 아래 정보로 로그인하세요.',
			relay: '릴레이 주소',
			npub: '내 주소 (npub)',
			npubHint: '공유 가능 — 남이 나를 초대하거나 멘션할 때 씁니다.',
			nsec: '내 키 (nsec)',
			reveal: '키 보기',
			warning: '이 키는 계정 전체에 대한 접근 권한입니다. 누구에게도 공유하지 마세요.',
			scanHint: '모바일 Buzz 앱에서 이 QR을 스캔해 키를 가져올 수 있습니다.',
			locked: '먼저 로그인해 신원을 잠금 해제하세요.'
		},
		en: {
			title: 'Connect Buzz app',
			description: 'Sign in to a mobile or desktop Buzz app with the details below.',
			relay: 'Relay address',
			npub: 'Your address (npub)',
			npubHint: 'Shareable — others use it to invite or mention you.',
			nsec: 'Your key (nsec)',
			reveal: 'Reveal key',
			warning: 'This key grants full access to your account. Never share it with anyone.',
			scanHint: 'Scan this QR in a mobile Buzz app to import your key.',
			locked: 'Sign in first to unlock your identity.'
		}
	});

	let relayURL = $state('');
	let revealed = $state(false);

	const secretHex = $derived(buzzIdentity.secretHex);
	const npub = $derived(secretHex ? npubEncode(buzzPublicKeyOf(secretHex)) : '');
	const nsec = $derived(secretHex ? nsecEncode(hexToBytes(secretHex)) : '');

	function hexToBytes(hex: string): Uint8Array {
		const bytes = new Uint8Array(hex.length / 2);
		for (let index = 0; index < bytes.length; index++) {
			bytes[index] = Number.parseInt(hex.slice(index * 2, index * 2 + 2), 16);
		}
		return bytes;
	}

	async function loadRelayURL() {
		try {
			const document: { relayURL?: string } = await fetch('/agent/api/buzz-relay-config', {
				credentials: 'include'
			}).then((response) => response.json());
			relayURL = document.relayURL?.trim() ?? '';
		} catch {
			relayURL = '';
		}
	}

	$effect(() => {
		if (!open) {
			revealed = false;
			return;
		}
		if (!relayURL) loadRelayURL();
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{text.title}</Dialog.Title>
			<Dialog.Description>{text.description}</Dialog.Description>
		</Dialog.Header>

		{#if !secretHex}
			<p class="text-muted-foreground rounded-md border bg-muted/30 px-3 py-2 text-sm">{text.locked}</p>
		{:else}
			<div class="flex flex-col gap-4">
				<div class="flex flex-col gap-1.5">
					<Label>{text.relay}</Label>
					<div class="flex items-center gap-2">
						<Input readonly value={relayURL} class="font-mono text-xs" />
						<CopyButton text={relayURL} variant="outline" />
					</div>
				</div>

				<div class="flex flex-col gap-1.5">
					<Label>{text.npub}</Label>
					<div class="flex items-center gap-2">
						<Input readonly value={npub} class="font-mono text-xs" />
						<CopyButton text={npub} variant="outline" />
					</div>
					<p class="text-muted-foreground text-xs">{text.npubHint}</p>
				</div>

				<div class="flex flex-col gap-1.5">
					<Label>{text.nsec}</Label>
					{#if !revealed}
						<Button variant="outline" class="gap-2" onclick={() => (revealed = true)}>
							<EyeIcon class="size-4" />
							{text.reveal}
						</Button>
					{:else}
						<div class="flex items-center gap-2">
							<Input readonly value={nsec} class="font-mono text-xs" />
							<CopyButton text={nsec} variant="outline" />
						</div>
						<p class="text-destructive flex items-start gap-1.5 text-xs">
							<TriangleAlertIcon class="mt-0.5 size-3.5 shrink-0" />
							<span>{text.warning}</span>
						</p>
						<div class="mt-2 flex flex-col items-center gap-2 rounded-lg border bg-muted/30 p-4">
							<QrCode value={nsec} size="160" />
							<p class="text-muted-foreground flex items-center gap-1.5 text-center text-xs">
								<SmartphoneIcon class="size-3.5 shrink-0" />
								{text.scanHint}
							</p>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
