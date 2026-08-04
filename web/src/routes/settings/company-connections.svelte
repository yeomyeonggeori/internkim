<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import {
		fetchCompanyConnections,
		forgetCompanyConnection,
		saveCompanyConnection,
		type CompanyConnection,
		type CompanyConnectionKind
	} from '$lib/company/connections';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	type Draft = { host: string; port: string; username: string; secret: string; hasSecret: boolean };

	const text = createPageText(companySettingsText);
	const kinds: CompanyConnectionKind[] = ['mattermost', 'smtp', 'imap', 'caldav'];
	const fieldID = $props.id();

	let drafts = $state<Record<string, Draft>>(emptyDrafts());
	let savingKind = $state('');
	let isLoading = $state(true);

	function emptyDrafts(): Record<string, Draft> {
		return Object.fromEntries(
			kinds.map((kind) => [kind, { host: '', port: '', username: '', secret: '', hasSecret: false }])
		);
	}

	function draftOf(connection: CompanyConnection): Draft {
		const settings = connection.settings as { port?: unknown; username?: unknown };
		return {
			host: connection.host,
			port: typeof settings.port === 'number' ? String(settings.port) : '',
			username: typeof settings.username === 'string' ? settings.username : '',
			secret: '',
			hasSecret: connection.hasSecret
		};
	}

	async function load() {
		isLoading = true;
		try {
			const connections = await fetchCompanyConnections();
			const loaded = emptyDrafts();
			for (const connection of connections) loaded[connection.kind] = draftOf(connection);
			drafts = loaded;
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.loadFailed);
		} finally {
			isLoading = false;
		}
	}

	async function save(kind: CompanyConnectionKind) {
		const draft = drafts[kind];
		savingKind = kind;
		try {
			await saveCompanyConnection({
				kind,
				host: draft.host.trim(),
				settings: {
					...(draft.port.trim() ? { port: Number(draft.port) } : {}),
					...(draft.username.trim() ? { username: draft.username.trim() } : {})
				},
				secret: draft.secret || undefined
			});
			toast.success(text.saved);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.saveFailed);
		} finally {
			savingKind = '';
		}
	}

	async function forget(kind: CompanyConnectionKind) {
		savingKind = kind;
		try {
			await forgetCompanyConnection(kind);
			await load();
		} catch (error) {
			toast.error(error instanceof Error ? error.message : text.saveFailed);
		} finally {
			savingKind = '';
		}
	}

	onMount(load);
</script>

<div class="grid gap-4">
	{#each kinds as kind (kind)}
		<Card.Root>
			<Card.Header>
				<Card.Title>{text[kind]}</Card.Title>
				<Card.Description>{text[`${kind}Description`]}</Card.Description>
			</Card.Header>
			<Card.Content class="grid gap-3 sm:grid-cols-2">
				<div class="grid gap-1.5">
					<Label for="{kind}-host-{fieldID}">{text.host}</Label>
					<Input id="{kind}-host-{fieldID}" bind:value={drafts[kind].host} disabled={isLoading} autocomplete="off" />
				</div>
				<div class="grid gap-1.5">
					<Label for="{kind}-port-{fieldID}">{text.port}</Label>
					<Input id="{kind}-port-{fieldID}" bind:value={drafts[kind].port} disabled={isLoading} inputmode="numeric" autocomplete="off" />
				</div>
				<div class="grid gap-1.5">
					<Label for="{kind}-username-{fieldID}">{text.username}</Label>
					<Input id="{kind}-username-{fieldID}" bind:value={drafts[kind].username} disabled={isLoading} autocomplete="off" />
				</div>
				<div class="grid gap-1.5">
					<Label for="{kind}-secret-{fieldID}">{text.password}</Label>
					<Input
						id="{kind}-secret-{fieldID}"
						type="password"
						bind:value={drafts[kind].secret}
						disabled={isLoading}
						placeholder={drafts[kind].hasSecret ? text.passwordKept : ''}
						autocomplete="new-password"
					/>
				</div>
			</Card.Content>
			<Card.Footer class="gap-2">
				<Button onclick={() => save(kind)} disabled={isLoading || savingKind === kind || !drafts[kind].host.trim()}>
					{text.save}
				</Button>
				{#if drafts[kind].host}
					<Button variant="outline" onclick={() => forget(kind)} disabled={savingKind === kind}>{text.forget}</Button>
				{/if}
			</Card.Footer>
		</Card.Root>
	{/each}
</div>
