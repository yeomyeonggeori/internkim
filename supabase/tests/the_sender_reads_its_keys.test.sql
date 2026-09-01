begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from vault.secrets where name in ('vapid_public_key', 'vapid_private_key', 'vapid_subject');

select is(
  (select count(*)::int from jsonb_each_text(public.vapid_keys_read()) where value is not null),
  0,
  'an unissued project hands the sender nothing'
);

select ok(
  public.vapid_keys_keep('pub-read', 'priv-read', 'mailto:read@example.com', false),
  'issuing stores a pair for the sender to read'
);

select is(
  public.vapid_keys_read(),
  '{"subject": "mailto:read@example.com", "publicKey": "pub-read", "privateKey": "priv-read"}'::jsonb,
  'the sender reads all three values'
);

select ok(
  not has_function_privilege('anon', 'public.vapid_keys_read()', 'execute')
    and not has_function_privilege('authenticated', 'public.vapid_keys_read()', 'execute'),
  'only the service reads the private key'
);

select * from finish();
rollback;
