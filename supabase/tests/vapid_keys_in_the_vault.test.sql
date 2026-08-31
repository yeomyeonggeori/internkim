begin;
create extension if not exists pgtap with schema extensions;
select plan(8);

select has_function(
  'public',
  'vapid_keys_keep',
  'the vault has a keeper for the VAPID keys'
);

select ok(
  public.vapid_keys_keep('pub-one', 'priv-one', 'mailto:one@example.com', false),
  'the first issuance stores the pair'
);

select is(
  (select count(*)::int from vault.secrets
   where name in ('vapid_public_key', 'vapid_private_key', 'vapid_subject')),
  3,
  'the vault holds the public key, the private key, and the subject'
);

select ok(
  not public.vapid_keys_keep('pub-two', 'priv-two', 'mailto:two@example.com', false),
  'a second issuance without rotation is a no-op'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
  'pub-one',
  'the first pair survives the no-op'
);

select ok(
  public.vapid_keys_keep('pub-two', 'priv-two', 'mailto:two@example.com', true),
  'rotation replaces the pair'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
  'pub-two',
  'rotation actually swapped the stored key'
);

select ok(
  not has_function_privilege('anon', 'public.vapid_keys_keep(text, text, text, boolean)', 'execute')
    and not has_function_privilege('authenticated', 'public.vapid_keys_keep(text, text, text, boolean)', 'execute'),
  'nobody signed in can write the VAPID keys directly'
);

select * from finish();
rollback;
