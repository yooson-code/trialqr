-- =====================================================================
--  Update 2: tanggal lahir (umur & kategori otomatis), data orang tua,
--  kartu RFID, dan metode presensi.
--  Jalankan SETELAH 001_init.sql. Aman dijalankan berulang (idempotent).
-- =====================================================================

alter table members add column if not exists birth_date   date;
alter table members add column if not exists parent_name  text not null default '';
alter table members add column if not exists parent_phone text not null default '';
alter table members add column if not exists rfid_uid     text;

do $$ begin
  if not exists (select 1 from pg_constraint where conname = 'members_birth_not_future') then
    alter table members add constraint members_birth_not_future
      check (birth_date is null or birth_date <= current_date);
  end if;
end $$;

-- satu kartu RFID hanya boleh dimiliki satu anggota (NULL boleh banyak)
create unique index if not exists members_rfid_uq   on members (rfid_uid) where rfid_uid is not null;
create index        if not exists members_birth_idx on members (birth_date);

-- cara absen dicatat: qr | rfid | manual
alter table attendances add column if not exists method text not null default 'qr';
do $$ begin
  if not exists (select 1 from pg_constraint where conname = 'attendances_method_chk') then
    alter table attendances add constraint attendances_method_chk
      check (method in ('qr','rfid','manual'));
  end if;
end $$;
