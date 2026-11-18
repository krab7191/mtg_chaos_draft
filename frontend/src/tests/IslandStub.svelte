<script lang="ts">
  // Stand-in for a page's Svelte island in page tests: renders the props it
  // was given, so a test can check what the page passed in without rendering
  // (and coverage-counting) the real component on the server.
  let props = $props();
</script>

<pre class="island-stub">{JSON.stringify(props)}</pre>
