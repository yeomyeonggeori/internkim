begin;
create extension if not exists pgtap with schema extensions;
select plan(4);

delete from public.company;

insert into public.company (id, name, slug, country, locale, timezone) values
  ('7b000000-0000-0000-0000-0000000000c1', 'Company A', 'company-a', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (company_id, email, status)
select '7b000000-0000-0000-0000-0000000000c1', 'person' || index || '@example.com', 'active'
from generate_series(1, 400) as index;
insert into public.member (company_id, email, status)
values ('7b000000-0000-0000-0000-0000000000c1', 'left@example.com', 'disabled');

select public.keep_claimed_box('7b000000-0000-0000-0000-0000000000c1', 'small', repeat('s', 43), null, false);
select public.keep_claimed_box('7b000000-0000-0000-0000-0000000000c1', 'large', repeat('l', 43), null, false);
select public.keep_claimed_box('7b000000-0000-0000-0000-0000000000c1', 'unplugged', repeat('u', 43), null, false);
update public.box set weight = 3, last_seen_at = now() where public_key = 'large';
update public.box set last_seen_at = now() where public_key = 'small';
update public.box set last_seen_at = now() - interval '1 hour' where public_key = 'unplugged';

create temporary table placed_before as select * from public.browser_homes('7b000000-0000-0000-0000-0000000000c1');

select is((select count(*)::integer from placed_before), 400, 'every active member has one browser home, and a member who left has none');
select ok(
  (select count(*) from placed_before where public_key = 'large') between 260 and 340,
  'a box of three times the weight runs about three quarters of the browsers'
);
select is((select count(*)::integer from placed_before where public_key = 'unplugged'), 0, 'a box not seen lately runs none');

update public.box set last_seen_at = now() where public_key = 'unplugged';
select is(
  (select count(*)::integer from public.browser_homes('7b000000-0000-0000-0000-0000000000c1') as placed_after
    join placed_before using (member_email)
    where placed_after.public_key <> placed_before.public_key and placed_after.public_key <> 'unplugged'),
  0,
  'a box that comes back takes only the browsers it wins, and moves none between the others'
);

select * from finish();
rollback;
