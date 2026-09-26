begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

select ok(
  not has_table_privilege('anon', 'public.empty_box', 'select'),
  'an anonymous caller cannot list empty boxes'
);

select ok(
  not has_table_privilege('authenticated', 'public.empty_box', 'select'),
  'a signed-in member cannot list empty boxes except through the route that matches their address'
);

select ok(
  has_table_privilege('service_role', 'public.empty_box', 'insert'),
  'the server records an announcing box'
);

select throws_ok(
  $$insert into public.empty_box (public_key, encryption_key, public_address)
    values ('too-short', 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM', '198.51.100.1')$$,
  '23514',
  null,
  'a public key is a 32-byte key in base64url'
);

select lives_ok(
  $$insert into public.empty_box (public_key, encryption_key, public_address)
    values ('ERERERERERERERERERERERERERERERERERERERERERE', 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM', '198.51.100.1')$$,
  'a box announces with its two keys and the address it came from'
);

select * from finish();
rollback;
