<script>
	import { onMount } from 'svelte';

	let isResolved = $state(false);

	onMount(() => {
		isResolved = true;
	});
</script>

<p data-gate>{isResolved ? 'signed-in-check-resolved' : 'checking-sign-in'}</p>
