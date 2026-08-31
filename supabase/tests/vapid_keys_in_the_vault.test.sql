begin;
create extension if not exists pgtap with schema extensions;
select plan(18);

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

delete from vault.secrets where name = 'vapid_subject';

select ok(
  not public.vapid_keys_keep('pub-three', 'priv-three', 'mailto:three@example.com', false),
  'a standing pair with no subject is still not an issuance'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
  'pub-two',
  'the standing public key survives a missing subject'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_private_key'),
  'priv-two',
  'the standing private key survives a missing subject'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_subject'),
  'mailto:three@example.com',
  'the missing subject is filled in without touching the pair'
);

delete from vault.secrets where name in ('vapid_public_key', 'vapid_private_key', 'vapid_subject');

insert into auth.users (id, email) values
  ('64000000-0000-0000-0000-000000000011', 'vapidboss@example.test');

insert into public.company (id, name, slug, country, locale, timezone, work_locations) values
  ('64000000-0000-0000-0000-0000000000c1', 'Ours', 'ours-vapid', 'KR', 'ko', 'Asia/Seoul', null);

insert into public.member (id, company_id, email, user_id, status, is_admin) values
  ('64000000-0000-0000-0000-0000000000a1', '64000000-0000-0000-0000-0000000000c1',
   'vapidboss@example.test', '64000000-0000-0000-0000-000000000011', 'active', true);

insert into public.push_device (member_id, kind, address, keys) values
  ('64000000-0000-0000-0000-0000000000a1', 'web-push', 'https://push.example.com/standing',
   '{"p256dh":"x","auth":"y"}'::jsonb);

select throws_ok(
  $$select public.vapid_keys_keep('pub-four', 'priv-four', 'mailto:four@example.com', false)$$,
  'P0001',
  'a web-push device stands and this vault cannot sign for it',
  'a first issuance refuses while devices carry a key the vault never held'
);

select is(
  (select count(*)::int from vault.secrets where name = 'vapid_public_key'),
  0,
  'the refused issuance stored nothing'
);

select ok(
  (select vault.create_secret('pub-half', 'vapid_public_key')) is not null,
  'a half-kept vault is set up by writing only the public key'
);

select throws_ok(
  $$select public.vapid_keys_keep('pub-five', 'priv-five', 'mailto:five@example.com', false)$$,
  'P0001',
  'a web-push device stands and this vault cannot sign for it',
  'a half-kept vault refuses too, because overwriting the standing half orphans the same devices'
);

select is(
  (select decrypted_secret from vault.decrypted_secrets where name = 'vapid_public_key'),
  'pub-half',
  'the standing half survives the refusal'
);

select ok(
  public.vapid_keys_keep('pub-standing', 'priv-standing', 'mailto:standing@example.com', true),
  'the standing pair is adopted with replace_existing, which is how a running deployment moves in'
);

select * from finish();
rollback;
