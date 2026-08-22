begin;
create extension if not exists pgtap with schema extensions;
select plan(10);

insert into auth.users (id, email) values
  ('44000000-0000-0000-0000-000000000001', 'face-a@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('44000000-0000-0000-0000-0000000000a0', 'Face', 'face', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, name) values
  ('44000000-0000-0000-0000-0000000000b0', '44000000-0000-0000-0000-0000000000a0', 'face-a@example.test', '44000000-0000-0000-0000-000000000001', '이샘플');

insert into public.contact (id, company_id, name) values
  ('44000000-0000-0000-0000-0000000000c0', '44000000-0000-0000-0000-0000000000a0', '박예시');

-- A face is readable by everyone in the company, which is what shared means.
select lives_ok($$
  update public.company set profile_image = '44000000-0000-0000-0000-0000000000a0/shared/face/company.png'
  where id = '44000000-0000-0000-0000-0000000000a0'
$$, 'a company keeps a picture in its own shared scope');

select throws_ok($$
  update public.member set profile_image = '44000000-0000-0000-0000-0000000000a0/person/44000000-0000-0000-0000-0000000000b0/face.png'
  where id = '44000000-0000-0000-0000-0000000000b0'
$$, '23514', null, 'a face kept where only one person may read it is refused');

select throws_ok($$
  update public.contact set profile_image = 'not-a-path'
  where id = '44000000-0000-0000-0000-0000000000c0'
$$, '23514', null, 'a path outside the bucket convention is refused');

select throws_ok($$
  update public.contact set profile_image = '44000000-0000-0000-0000-0000000000ff/shared/face/other.png'
  where id = '44000000-0000-0000-0000-0000000000c0'
$$, '23514', null, 'a face kept under another company is refused');

insert into storage.objects (bucket_id, name, owner) values
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/face/member.png', null),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/face/contact.png', null),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/face/kept.png', null);

update public.member set profile_image = '44000000-0000-0000-0000-0000000000a0/shared/face/member.png'
where id = '44000000-0000-0000-0000-0000000000b0';
update public.contact set profile_image = '44000000-0000-0000-0000-0000000000a0/shared/face/contact.png'
where id = '44000000-0000-0000-0000-0000000000c0';

-- A picture whose row is gone is one nobody can reach and nobody would delete.
delete from public.member where id = '44000000-0000-0000-0000-0000000000b0';
select isnt_empty($$
  select 1 from public.abandoned_asset where path = '44000000-0000-0000-0000-0000000000a0/shared/face/member.png'
$$, 'a departed member leaves their picture to be collected');

delete from public.contact where id = '44000000-0000-0000-0000-0000000000c0';
select isnt_empty($$
  select 1 from public.abandoned_asset where path = '44000000-0000-0000-0000-0000000000a0/shared/face/contact.png'
$$, 'a removed contact leaves their picture to be collected');

select is_empty($$
  select 1 from public.abandoned_asset where path = '44000000-0000-0000-0000-0000000000a0/shared/face/kept.png'
$$, 'a picture nobody abandoned is not queued for collection');

-- Replacing a picture orphans the one it replaces just as surely as deleting it.
insert into storage.objects (bucket_id, name, owner) values
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/face/old.png', null),
  ('asset', '44000000-0000-0000-0000-0000000000a0/shared/face/new.png', null);
update public.company set profile_image = '44000000-0000-0000-0000-0000000000a0/shared/face/old.png'
where id = '44000000-0000-0000-0000-0000000000a0';
update public.company set profile_image = '44000000-0000-0000-0000-0000000000a0/shared/face/new.png'
where id = '44000000-0000-0000-0000-0000000000a0';

select isnt_empty($$
  select 1 from public.abandoned_asset where path = '44000000-0000-0000-0000-0000000000a0/shared/face/old.png'
$$, 'a replaced picture is queued for collection too');

select is_empty($$
  select 1 from public.abandoned_asset where path = '44000000-0000-0000-0000-0000000000a0/shared/face/new.png'
$$, 'the picture that replaced it is not');

-- The queue names every company's paths, and a member has no business reading
-- another company's out of it.
select is(
  (select count(*)::int from pg_policies where schemaname = 'public' and tablename = 'abandoned_asset'),
  0,
  'the collection queue carries no policy, so only the key that sweeps it can read it'
);

select * from finish();
rollback;
