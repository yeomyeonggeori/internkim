begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into public.company (id, name, slug, country, locale, timezone) values
  ('45000000-0000-0000-0000-0000000000a0', 'Seal', 'seal', 'KR', 'ko', 'Asia/Seoul');

select lives_ok($$
  update public.company set seal_image = '45000000-0000-0000-0000-0000000000a0/shared/company/seal-a.png'
  where id = '45000000-0000-0000-0000-0000000000a0'
$$, 'a company keeps its seal in the company folder of its shared scope');

select throws_ok($$
  update public.company set seal_image = '45000000-0000-0000-0000-0000000000a0/shared/attachment/seal.png'
  where id = '45000000-0000-0000-0000-0000000000a0'
$$, '23514', null, 'a seal outside the company folder, which only an administrator writes, is refused');

select throws_ok($$
  update public.company set seal_image = '45000000-0000-0000-0000-0000000000a0/person/45000000-0000-0000-0000-0000000000b0/seal.png'
  where id = '45000000-0000-0000-0000-0000000000a0'
$$, '23514', null, 'a seal kept where only one person may read it is refused');

select throws_ok($$
  update public.company set seal_image = '45000000-0000-0000-0000-0000000000ff/shared/company/seal.png'
  where id = '45000000-0000-0000-0000-0000000000a0'
$$, '23514', null, 'a seal kept under another company is refused');

update public.company set seal_image = '45000000-0000-0000-0000-0000000000a0/shared/company/seal-b.png'
where id = '45000000-0000-0000-0000-0000000000a0';

select isnt_empty($$
  select 1 from public.abandoned_asset where path = '45000000-0000-0000-0000-0000000000a0/shared/company/seal-a.png'
$$, 'a replaced seal is queued for collection');

select is_empty($$
  select 1 from public.abandoned_asset where path = '45000000-0000-0000-0000-0000000000a0/shared/company/seal-b.png'
$$, 'the seal that replaced it is not');

delete from public.company where id = '45000000-0000-0000-0000-0000000000a0';
select isnt_empty($$
  select 1 from public.abandoned_asset where path = '45000000-0000-0000-0000-0000000000a0/shared/company/seal-b.png'
$$, 'a removed company leaves its seal to be collected');

select * from finish();
rollback;
