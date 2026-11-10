export const heldWhileSignedInAttribute = 'data-landing-held';

const supabaseSessionKey = /^sb-.+-auth-token$/;

export const holdWhileSignedInScript = `<script>try{if(Object.keys(localStorage).some(function(key){return ${supabaseSessionKey}.test(key)}))document.documentElement.setAttribute('${heldWhileSignedInAttribute}','')}catch(error){}</script>`;
