<script lang="ts">
	import '../../app.css';
	import { onMount } from 'svelte';
	import { isSupabaseConfigured, supabase } from '$lib/supabase';
	import {
		fetchCompany,
		fetchMember,
		fetchMyAttendance,
		nextClockKind,
		recordAttendance,
		type AttendanceEntry,
		type Company,
		type Member,
	} from '$lib/clock';

	let signedInEmail = $state<string | null>(null);
	let email = $state('');
	let password = $state('');
	let member = $state<Member | null>(null);
	let company = $state<Company | null>(null);
	let entries = $state<AttendanceEntry[]>([]);
	let selectedLocation = $state<string | null>(null);
	let notice = $state<string | null>(null);
	let isBusy = $state(false);

	const pendingKind = $derived(nextClockKind(entries));
	const locations = $derived(company?.work_locations ?? []);

	function report(errorValue: unknown): void {
		notice = errorValue instanceof Error ? errorValue.message : String(errorValue);
	}

	async function loadEverything(): Promise<void> {
		member = await fetchMember();
		company = await fetchCompany();
		selectedLocation ??= locations[0] ?? null;
		entries = member ? await fetchMyAttendance(member.id) : [];
	}

	async function signIn(): Promise<void> {
		isBusy = true;
		notice = null;
		try {
			const { error } = await supabase().auth.signInWithPassword({ email, password });
			if (error) throw new Error(error.message);
		} catch (errorValue) {
			report(errorValue);
		} finally {
			isBusy = false;
		}
	}

	async function signOut(): Promise<void> {
		await supabase().auth.signOut();
		member = null;
		company = null;
		entries = [];
	}

	async function clock(): Promise<void> {
		if (!member) return;
		isBusy = true;
		notice = null;
		try {
			await recordAttendance(member.id, pendingKind, selectedLocation);
			entries = await fetchMyAttendance(member.id);
		} catch (errorValue) {
			report(errorValue);
		} finally {
			isBusy = false;
		}
	}

	function formatTime(occurredAt: string): string {
		const zone = member?.timezone ?? company?.timezone;
		return new Date(occurredAt).toLocaleString(undefined, zone ? { timeZone: zone } : undefined);
	}

	onMount(() => {
		if (!isSupabaseConfigured) {
			notice = 'set VITE_SUPABASE_URL and VITE_SUPABASE_PUBLISHABLE_KEY';
			return;
		}
		const { data } = supabase().auth.onAuthStateChange((_event, session) => {
			signedInEmail = session?.user.email ?? null;
			if (session) loadEverything().catch(report);
		});
		supabase()
			.auth.getSession()
			.then(({ data: current }) => {
				signedInEmail = current.session?.user.email ?? null;
				if (current.session) loadEverything().catch(report);
			});
		return () => data.subscription.unsubscribe();
	});
</script>

<svelte:head><title>Clock</title></svelte:head>

<main class="mx-auto flex max-w-md flex-col gap-6 p-8">
	<h1 class="text-xl font-semibold">{company?.name ?? 'InternKim'}</h1>

	{#if !signedInEmail}
		<form class="flex flex-col gap-3" onsubmit={(event) => { event.preventDefault(); signIn(); }}>
			<input class="rounded border p-2" type="email" bind:value={email} placeholder="email" required />
			<input class="rounded border p-2" type="password" bind:value={password} placeholder="password" required />
			<button class="rounded bg-primary p-2 text-primary-foreground" disabled={isBusy}>Sign in</button>
		</form>
	{:else if !member}
		<p class="text-sm text-muted-foreground">
			{signedInEmail} is signed in but is not a member of any company yet.
		</p>
		<button class="text-sm underline" onclick={signOut}>Sign out</button>
	{:else}
		<div class="flex items-center justify-between text-sm text-muted-foreground">
			<span>{signedInEmail}</span>
			<button class="underline" onclick={signOut}>Sign out</button>
		</div>

		{#if pendingKind === 'clock_in' && locations.length > 0}
			<select class="rounded border p-2" bind:value={selectedLocation}>
				{#each locations as location (location)}
					<option value={location}>{location}</option>
				{/each}
			</select>
		{/if}

		<button class="rounded bg-primary p-3 text-primary-foreground" disabled={isBusy} onclick={clock}>
			{pendingKind === 'clock_in' ? 'Clock in' : 'Clock out'}
		</button>

		<ul class="flex flex-col gap-1 text-sm">
			{#each entries as entry (entry.id)}
				<li class="flex justify-between border-b py-1">
					<span>{entry.kind === 'clock_in' ? 'in' : 'out'}{entry.location ? ` · ${entry.location}` : ''}</span>
					<span class="text-muted-foreground">{formatTime(entry.occurred_at)}</span>
				</li>
			{/each}
		</ul>
	{/if}

	{#if notice}
		<p class="text-sm text-destructive">{notice}</p>
	{/if}
</main>
