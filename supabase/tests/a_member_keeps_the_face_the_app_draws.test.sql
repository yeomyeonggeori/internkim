begin;
create extension if not exists pgtap with schema extensions;
select plan(7);

insert into auth.users (id, email) values
  ('45000000-0000-0000-0000-000000000001', 'drawn-a@example.test'),
  ('45000000-0000-0000-0000-000000000002', 'drawn-b@example.test');

insert into public.company (id, name, slug, country, locale, timezone) values
  ('45000000-0000-0000-0000-0000000000a0', '샘플회사', 'drawn-sample', 'KR', 'ko', 'Asia/Seoul'),
  ('45000000-0000-0000-0000-0000000000a1', '예시회사', 'drawn-example', 'KR', 'ko', 'Asia/Seoul');

insert into public.member (id, company_id, email, user_id, name, profile_image) values
  ('45000000-0000-0000-0000-0000000000b0', '45000000-0000-0000-0000-0000000000a0', 'drawn-a@example.test', '45000000-0000-0000-0000-000000000001', '이샘플', null),
  ('45000000-0000-0000-0000-0000000000b1', '45000000-0000-0000-0000-0000000000a0', 'drawn-b@example.test', '45000000-0000-0000-0000-000000000002', '박예시', '45000000-0000-0000-0000-0000000000a0/shared/face/chosen.png');

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  assert public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png'),
    'a member with no picture was not given the drawn one';
  assert not public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a0/shared/member/again.png'),
    'a second drawing replaced the first';
  reset role;
end $$;$block$, 'a member with no picture keeps the drawn one, once');

select is(
  (select profile_image from public.member where id = '45000000-0000-0000-0000-0000000000b0'),
  '45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png',
  'the first drawing is the one kept'
);

select lives_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000002"}', true);
  assert not public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a0/shared/member/drawn-b.png'),
    'a drawing replaced a chosen picture';
  reset role;
end $$;$block$, 'a picture already chosen is left alone');

select is(
  (select profile_image from public.member where id = '45000000-0000-0000-0000-0000000000b1'),
  '45000000-0000-0000-0000-0000000000a0/shared/face/chosen.png',
  'the chosen picture is still there'
);

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  perform public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a1/shared/member/other.png');
end $$;$block$, '22023', null, 'a drawing kept under another company is refused');

select throws_ok($block$do $$
begin
  set local role authenticated;
  perform set_config('request.jwt.claims', '{"sub":"45000000-0000-0000-0000-000000000001"}', true);
  perform public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a0/shared/company/logo.png');
end $$;$block$, '22023', null, 'a drawing kept outside the member folder is refused');

select throws_ok($block$do $$
begin
  set local role anon;
  perform set_config('request.jwt.claims', '{}', true);
  perform public.member_picture_keep_drawn('45000000-0000-0000-0000-0000000000a0/shared/member/drawn.png');
end $$;$block$, '42501', null, 'a signed-out caller cannot keep a drawing');

select * from finish();
rollback;
