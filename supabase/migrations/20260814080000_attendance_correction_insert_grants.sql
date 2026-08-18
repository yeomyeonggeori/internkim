revoke insert on public.attendance from authenticated;

grant insert (member_id, kind, location) on public.attendance to authenticated;
