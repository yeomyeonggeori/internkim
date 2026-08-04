<script lang="ts">
	import BuzzMessenger from './buzz-messenger.svelte';
	import CompanyMessenger from './company-messenger.svelte';
	import { isSupabaseConfigured } from '$lib/supabase';
</script>

{#if isSupabaseConfigured()}
	<CompanyMessenger />
{:else}
	<BuzzMessenger />
{/if}
