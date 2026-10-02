revoke all on public.data_room_link, public.data_room_link_session, public.data_room_link_attempt
  from anon, authenticated;

grant select (id, company_id, creator_member_id, role_code, label, can_download, created_at, expires_at, revoked_at)
  on public.data_room_link to authenticated;
