<script lang="ts">
	import { onMount } from 'svelte';
	import { z } from 'zod';
	import { page } from '$app/state';
	import * as InputOTP from '$lib/components/ui/input-otp';
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { Spinner } from '$lib/components/ui/spinner';
	import * as Field from '$lib/components/ui/field';
	import FileBrowserList from '$lib/components/file-browser-list.svelte';
	import FileBrowserPreview from '$lib/components/file-browser-preview.svelte';
	import { sharedDataRoomSchema } from '$lib/data-room/schemas';
	import { dataRoomNoticeVersion } from '$lib/data-room/links';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { dataRoomSharingText } from '$lib/data-room/sharing-text';
	import { dataRoomText } from '$lib/data-room/text';
	const sharedLinkSchema = sharedDataRoomSchema.extend({ canDownload: z.boolean() });
	type SharedRoom = z.infer<typeof sharedLinkSchema>;
	type SharedDocument = SharedRoom['documents'][number];
	const text = createPageText(dataRoomSharingText);
	const browserText = createPageText(dataRoomText);
	const fieldID = $props.id();
	let accessCode = $state('');
	let hasAccepted = $state(false);
	let isBusy = $state(false);
	let errorMessage = $state('');
	let room = $state<SharedRoom | null>(null);
	let selectedCode = $state('');
	let selectedDocument = $state<SharedDocument | null>(null);
	const endpoint = $derived(`/api/v1/data-room/links/${page.params.linkID}`);
	const categories = $derived(room?.categories ?? []);
	const entries = $derived(
		(room?.documents ?? [])
			.filter(
				(document) =>
					!selectedCode ||
					document.category_code === selectedCode ||
					categories.find((category) => category.code === document.category_code)?.parent ===
						selectedCode
			)
			.map((document) => ({
				id: document.id,
				name: document.title,
				secondary: document.category_code,
				date: document.document_date ?? ''
			}))
	);

	function categoryLabel(code: string): string {
		const category = categories.find((candidate) => candidate.code === code);
		return currentLocale.value === 'ko'
			? category?.name_ko || category?.name || code
			: category?.name || code;
	}

	async function readResponse(response: Response): Promise<unknown> {
		const answer: unknown = await response.json();
		if (response.ok) return answer;
		if (response.status === 401 || response.status === 403) {
			room = null;
			selectedDocument = null;
		}
		const refusal = z.object({ message: z.string() }).safeParse(answer);
		throw new Error(refusal.success ? refusal.data.message : text.failure);
	}

	async function load() {
		room = sharedLinkSchema.parse(await readResponse(await fetch(endpoint, { cache: 'no-store' })));
		if (selectedDocument)
			selectedDocument =
				room.documents.find((document) => document.id === selectedDocument?.id) ?? null;
	}

	async function unlock(event: SubmitEvent) {
		event.preventDefault();
		if (!hasAccepted) return;
		isBusy = true;
		errorMessage = '';
		try {
			await readResponse(
				await fetch(endpoint, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ accessCode, noticeVersion: dataRoomNoticeVersion })
				})
			);
			accessCode = '';
			await load();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function download(documentID: string) {
		isBusy = true;
		errorMessage = '';
		try {
			const answer = z
				.object({ downloadURL: z.string().url() })
				.parse(
					await readResponse(
						await fetch(`${endpoint}?${new URLSearchParams({ documentID })}`, { cache: 'no-store' })
					)
				);
			window.location.assign(answer.downloadURL);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		} finally {
			isBusy = false;
		}
	}

	async function refresh() {
		try {
			await load();
			errorMessage = '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failure;
		}
	}

	onMount(() => {
		const timer = window.setInterval(() => {
			if (room && !isBusy) void refresh();
		}, 30000);
		return () => window.clearInterval(timer);
	});
</script>

<svelte:head
	><title>{browserText.title}</title><meta name="robots" content="noindex,nofollow" /><meta
		name="referrer"
		content="no-referrer"
	/></svelte:head
>

<main class="mx-auto w-full max-w-6xl p-4 sm:p-8">
	{#if !room}
		<section class="mx-auto grid max-w-md gap-6 py-8">
			<header class="grid gap-2">
				<h1 class="text-xl font-semibold">{text.enterCode}</h1>
				<p class="text-sm text-muted-foreground">{text.unlockDescription}</p>
			</header>
			<form onsubmit={unlock} class="grid gap-5">
				<Field.Field>
					<Field.Label for="{fieldID}-code">{text.code}</Field.Label>
					<InputOTP.Root
						inputId="{fieldID}-code"
						type="password"
						inputmode="numeric"
						autocomplete="one-time-code"
						pattern="[0-9]*"
						minlength={6}
						maxlength={6}
						required
						bind:value={accessCode}
						disabled={isBusy}
					>
						{#snippet children({ cells })}
							<InputOTP.Group>
								{#each cells.slice(0, 3) as cell (cell)}
									<InputOTP.Slot cell={{ ...cell, char: cell.char ? '•' : cell.char }} />
								{/each}
							</InputOTP.Group>
							<InputOTP.Separator />
							<InputOTP.Group>
								{#each cells.slice(3, 6) as cell (cell)}
									<InputOTP.Slot cell={{ ...cell, char: cell.char ? '•' : cell.char }} />
								{/each}
							</InputOTP.Group>
						{/snippet}
					</InputOTP.Root>
				</Field.Field>
				<section class="grid gap-2 rounded-lg border p-4" aria-label={text.noticeTitle}>
					<h2 class="text-sm font-semibold">{text.noticeTitle}</h2>
					<p class="text-sm leading-relaxed text-muted-foreground">{text.notice}</p>
				</section>
				<div class="flex items-start gap-3">
					<Checkbox id="{fieldID}-consent" bind:checked={hasAccepted} disabled={isBusy} /><label
						for="{fieldID}-consent"
						class="text-sm leading-relaxed">{text.consent}</label
					>
				</div>
				<Button type="submit" disabled={!hasAccepted || accessCode.length !== 6 || isBusy}
					>{#if isBusy}<Spinner />{/if}{text.unlock}</Button
				>
			</form>
		</section>
	{:else}
		<header class="mb-6 flex items-center justify-between gap-3">
			<h1 class="text-xl font-semibold">{browserText.title}</h1>
			<Button variant="outline" size="sm" onclick={refresh}>{browserText.refresh}</Button>
		</header>
		<div class="mb-4 flex flex-wrap gap-2">
			<Button
				size="sm"
				variant={!selectedCode ? 'secondary' : 'ghost'}
				onclick={() => {
					selectedCode = '';
					selectedDocument = null;
				}}>{browserText.allDocuments}</Button
			>{#each categories as category (category.code)}<Button
					size="sm"
					variant={selectedCode === category.code ? 'secondary' : 'ghost'}
					onclick={() => {
						selectedCode = category.code;
						selectedDocument = null;
					}}>{categoryLabel(category.code)}</Button
				>{/each}
		</div>
		<div class="flex items-start gap-4">
			<FileBrowserList
				{entries}
				title={browserText.title}
				nameLabel={browserText.name}
				secondaryLabel={browserText.category}
				isSecondaryBadge
				dateLabel={browserText.date}
				emptyLabel={browserText.empty}
				selectedID={selectedDocument?.id}
				onSelect={(entry) =>
					(selectedDocument = room?.documents.find((document) => document.id === entry.id) ?? null)}
			/>
			<FileBrowserPreview
				isOpen={selectedDocument !== null}
				title={selectedDocument?.title ?? browserText.title}
				onClose={() => (selectedDocument = null)}
				>{#if selectedDocument}<div class="flex items-start justify-between gap-3 border-b p-4">
						<h2 class="text-sm font-semibold">{selectedDocument.title}</h2>
						<Button variant="ghost" size="sm" onclick={() => (selectedDocument = null)}
							>{browserText.close}</Button
						>
					</div>
					<div class="grid gap-4 overflow-auto p-4">
						<p class="text-xs text-muted-foreground">
							{categoryLabel(selectedDocument.category_code)} · {selectedDocument.document_date ??
								''}
						</p>
						<p class="whitespace-pre-wrap text-sm">
							{selectedDocument.summary || browserText.noSummary}
						</p>
						{#if room.canDownload}<Button
								variant="outline"
								disabled={isBusy}
								onclick={() => selectedDocument && download(selectedDocument.id)}
								>{browserText.download}</Button
							>{/if}
					</div>{/if}</FileBrowserPreview
			>
		</div>
	{/if}
	{#if errorMessage}<p role="alert" class="mx-auto mt-4 max-w-md text-sm text-destructive">
			{errorMessage}
		</p>{/if}
</main>
