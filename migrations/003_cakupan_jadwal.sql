-- =====================================================================
--  Update 3: cakupan pengajian (daerah / desa / kelompok).
--  Jalankan SETELAH 001 dan 002. Aman diulang (idempotent).
--  Jadwal yang sudah ada otomatis menjadi pengajian 'daerah'.
-- =====================================================================

alter table schedules add column if not exists level      text   not null default 'daerah';
alter table schedules add column if not exists village_id bigint references villages(id) on delete restrict;
alter table schedules add column if not exists group_id   bigint references groups(id)   on delete restrict;

do $$ begin
  if not exists (select 1 from pg_constraint where conname = 'schedules_level_chk') then
    alter table schedules add constraint schedules_level_chk
      check (level in ('daerah','desa','kelompok'));
  end if;
  -- desa -> wajib village_id; kelompok -> wajib group_id (desa-nya diturunkan dari kelompok)
  if not exists (select 1 from pg_constraint where conname = 'schedules_scope_chk') then
    alter table schedules add constraint schedules_scope_chk check (
      (level = 'daerah'   and village_id is null     and group_id is null) or
      (level = 'desa'     and village_id is not null and group_id is null) or
      (level = 'kelompok' and group_id   is not null and village_id is null)
    );
  end if;
end $$;

create index if not exists schedules_village_idx on schedules (village_id) where village_id is not null;
create index if not exists schedules_group_idx   on schedules (group_id)   where group_id   is not null;
