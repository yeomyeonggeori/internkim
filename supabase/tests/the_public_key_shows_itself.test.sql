begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

delete from vault.secrets where name in ('vapid_public_key', 'vapid_private_key', 'vapid_subject');

select has_function(
  'public',
  'vapid_public_key',
  'the browser has somewhere to ask for the public key'
);

select is(
  public.vapid_public_key(),
  null,
  'an unissued project answers nothing rather than failing'
);

select ok(
  public.vapid_keys_keep('pub-shown', 'priv-hidden', 'mailto:shown@example.com', false),
  'issuing stores a key for the read to find'
);

select is(
  public.vapid_public_key(),
  'pub-shown',
  'the issued public key comes back'
);

select ok(
  not has_function_privilege('anon', 'public.vapid_public_key()', 'execute')
    and has_function_privilege('authenticated', 'public.vapid_public_key()', 'execute'),
  'signed-in readers only, and never the private key'
);

select * from finish();
rollback;
