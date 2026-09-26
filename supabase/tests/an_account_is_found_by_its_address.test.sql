begin;
create extension if not exists pgtap with schema extensions;
select plan(5);

insert into auth.users (id, email) values
  ('00000000-0000-0000-0000-00000000a0c1', 'found.by.address@example.test');

select is(
  public.account_of_address('found.by.address@example.test'),
  '00000000-0000-0000-0000-00000000a0c1'::uuid,
  'the account answers to its address'
);

select is(
  public.account_of_address('Found.By.Address@Example.test'),
  '00000000-0000-0000-0000-00000000a0c1'::uuid,
  'the address matches whatever case it is typed in'
);

select is(
  public.account_of_address('nobody.here@example.test'),
  null,
  'an address nobody signed up with answers nothing'
);

select ok(
  not has_function_privilege('authenticated', 'public.account_of_address(text)', 'execute'),
  'a signed-in member cannot look an account up by address'
);

select ok(
  not has_function_privilege('anon', 'public.account_of_address(text)', 'execute'),
  'an anonymous caller cannot look an account up by address'
);

select * from finish();
rollback;
