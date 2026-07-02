<script lang="ts">
	import { invoke } from '@tauri-apps/api/core';
	import { getCurrentWindow } from '@tauri-apps/api/window';
	import { onMount } from 'svelte';
	import { controlPlaneBaseURL } from './lib/sidecar';

	const currentWindow = getCurrentWindow();
	const handoffBridgeURL = `${controlPlaneBaseURL}/v1/browser/handoff`;

	type BrowserHandoffSnapshot = {
		active: boolean;
		handoffID: string;
		sessionID: string;
		message: string;
		origin: string;
	};

	let handoffSnapshot = $state<BrowserHandoffSnapshot>({
		active: false,
		handoffID: '',
		sessionID: '',
		message: '브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요.',
		origin: ''
	});
	let handoffOverlayError = $state('');
	let isCompletingHandoff = $state(false);

	onMount(() => {
		document.body.classList.add('handoff-overlay-body');
		void refreshHandoffOverlay();
		const overlayIntervalID = window.setInterval(() => {
			void refreshHandoffOverlay();
		}, 250);
		return () => {
			window.clearInterval(overlayIntervalID);
			document.body.classList.remove('handoff-overlay-body');
		};
	});

	async function refreshHandoffOverlay() {
		try {
			handoffSnapshot = await readHandoffSnapshot();
			handoffOverlayError = '';
			await invoke('sync_handoff_overlay', { isActive: handoffSnapshot.active });
		} catch (errorValue) {
			handoffOverlayError = errorValue instanceof Error ? errorValue.message : 'Browser handoff overlay failed';
			await currentWindow.hide();
		}
	}

	async function readHandoffSnapshot(): Promise<BrowserHandoffSnapshot> {
		const response = await fetch(handoffBridgeURL, { cache: 'no-store' });
		if (!response.ok) return inactiveHandoffSnapshot();
		return normalizeHandoffSnapshot(await response.json());
	}

	function normalizeHandoffSnapshot(value: unknown): BrowserHandoffSnapshot {
		if (!isRecord(value) || value.active !== true) return inactiveHandoffSnapshot();
		return {
			active: true,
			handoffID: readString(value.handoffID),
			sessionID: readString(value.sessionID),
			message: readString(value.message) || '브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요.',
			origin: readString(value.origin)
		};
	}

	function inactiveHandoffSnapshot(): BrowserHandoffSnapshot {
		return {
			active: false,
			handoffID: '',
			sessionID: '',
			message: '브라우저에서 필요한 작업을 마친 뒤 완료를 눌러주세요.',
			origin: ''
		};
	}

	function isRecord(value: unknown): value is Record<string, unknown> {
		return typeof value === 'object' && value !== null;
	}

	function readString(value: unknown): string {
		return typeof value === 'string' ? value.trim() : '';
	}

	async function completeBrowserHandoff() {
		if (!handoffSnapshot.active || isCompletingHandoff) return;
		isCompletingHandoff = true;
		handoffOverlayError = '';
		try {
			const response = await fetch(`${handoffBridgeURL}/complete`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					handoffID: handoffSnapshot.handoffID,
					sessionID: handoffSnapshot.sessionID,
					url: handoffSnapshot.origin || 'https://internkim.local/browser-handoff-complete',
					title: ''
				})
			});
			if (!response.ok) {
				throw new Error(await response.text());
			}
			handoffSnapshot = inactiveHandoffSnapshot();
			await invoke('sync_handoff_overlay', { isActive: false });
		} catch (errorValue) {
			handoffOverlayError = errorValue instanceof Error ? errorValue.message : 'Browser handoff completion failed';
		} finally {
			isCompletingHandoff = false;
		}
	}
</script>

<main class="handoff-overlay">
	<p>{handoffOverlayError || handoffSnapshot.message}</p>
	<button disabled={!handoffSnapshot.active || isCompletingHandoff} onclick={completeBrowserHandoff}>
		{isCompletingHandoff ? '연결 중' : '계속'}
	</button>
</main>
