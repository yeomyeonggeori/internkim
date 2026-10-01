begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from public.company;

insert into auth.users (id, email) values
  ('7b000000-0000-0000-0000-000000000011', 'stays@example.test'),
  ('7b000000-0000-0000-0000-000000000012', 'withdrawn@example.test'),
  ('7b000000-0000-0000-0000-000000000013', 'departed@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7b000000-0000-0000-0000-0000000000c1', 'Company L', 'company-l', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, status) values
  ('7b000000-0000-0000-0000-0000000000a1', '7b000000-0000-0000-0000-0000000000c1',
   'stays@example.test', '7b000000-0000-0000-0000-000000000011', 'active'),
  ('7b000000-0000-0000-0000-0000000000a2', '7b000000-0000-0000-0000-0000000000c1',
   'withdrawn@example.test', '7b000000-0000-0000-0000-000000000012', 'withdrawn'),
  ('7b000000-0000-0000-0000-0000000000a3', '7b000000-0000-0000-0000-0000000000c1',
   'departed@example.test', '7b000000-0000-0000-0000-000000000013', 'departed');

insert into public.task (company_id, title) values
  ('7b000000-0000-0000-0000-0000000000c1', 'Quarterly plan');

insert into public.contact (company_id, name) values
  ('7b000000-0000-0000-0000-0000000000c1', 'A customer');

select lives_ok($block$do $$
declare
  written integer;
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7b000000-0000-0000-0000-000000000012"}', true);

  assert public.my_member() is null, 'a withdrawn member is still somebody here';
  assert (select count(*) from public.task) = 0, 'a withdrawn member still reads the tasks';
  assert (select count(*) from public.member) = 0, 'a withdrawn member still reads the directory';
  assert (select count(*) from public.contact) = 0, 'a withdrawn member still reads the contacts';

  begin
    insert into public.contact (company_id, name)
    values ('7b000000-0000-0000-0000-0000000000c1', 'written after leaving');
    written := 1;
  exception when insufficient_privilege then
    written := 0;
  end;
  assert written = 0, 'a withdrawn member still writes a contact';

  reset role;
end $$;$block$, 'a withdrawn member reads and writes nothing');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7b000000-0000-0000-0000-000000000013"}', true);

  assert public.my_member() is null, 'a departed member is still somebody here';
  assert (select count(*) from public.task) = 0, 'a departed member still reads the tasks';
  assert (select count(*) from public.member) = 0, 'a departed member still reads the directory';

  reset role;
end $$;$block$, 'a departed member reads nothing');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"7b000000-0000-0000-0000-000000000011"}', true);

  assert public.my_member() = '7b000000-0000-0000-0000-0000000000a1', 'a member who stays lost themselves';
  assert (select count(*) from public.task) = 1, 'a member who stays lost the tasks';
  assert (select count(*) from public.member) = 3, 'a member who stays lost the colleagues who left';

  reset role;
end $$;$block$, 'a member who stays still reads the company, the people who left included');

select is(
  (select count(*)::integer from public.contact where name = 'written after leaving'),
  0,
  'nothing a member who left tried to write was kept'
);

select * from finish();
rollback;
