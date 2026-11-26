-- =====================================================================
--  Update 4: status pernikahan (usia mandiri) dan laporan pra-musyawarah
--  bulanan. Jalankan SETELAH 001, 002, 003. Aman diulang (idempotent).
-- =====================================================================

-- ---------- Status pernikahan (hanya dipakai untuk kategori usia mandiri 19-30) ----------
alter table members add column if not exists marital_status text;
do $$ begin
  if not exists (select 1 from pg_constraint where conname = 'members_marital_chk') then
    alter table members add constraint members_marital_chk
      check (marital_status in ('belum','menikah'));
  end if;
end $$;

-- ---------- Laporan kinerja & kesiapan musyawarah bulanan ----------
create table if not exists monthly_reports (
    id               bigserial primary key,
    period           date not null check (extract(day from period) = 1),   -- selalu tanggal 1 bulan yang dilaporkan
    author_id        bigint references admins(id) on delete set null,
    author_name      text not null,                                        -- snapshot nama saat laporan dibuat
    bidang           text not null check (length(trim(bidang)) > 0),       -- bidang / jabatan pelapor
    level            text not null default 'daerah' check (level in ('daerah','desa','kelompok')),
    village_id       bigint references villages(id) on delete restrict,
    group_id         bigint references groups(id)   on delete restrict,
    summary          text not null default '',                             -- ringkasan umum kinerja
    items            jsonb not null default '[]'::jsonb,                   -- daftar kegiatan: nama, status, capaian, kendala, tindak lanjut
    attendance_note  text not null default '',                             -- catatan/pelengkap rekap kehadiran
    evaluation       text not null default '',                             -- poin evaluasi
    budget_needs     text not null default '',                             -- kebutuhan anggaran / logistik
    topics           text not null default '',                             -- topik yang ingin dibahas
    status           text not null default 'draft' check (status in ('draft','submitted')),
    submitted_at     timestamptz,
    created_at       timestamptz not null default now(),
    updated_at       timestamptz not null default now(),
    constraint monthly_reports_scope_chk check (
      (level = 'daerah'   and village_id is null     and group_id is null) or
      (level = 'desa'     and village_id is not null and group_id is null) or
      (level = 'kelompok' and group_id   is not null and village_id is null)
    ),
    -- satu laporan per pengurus per bulan
    constraint monthly_reports_author_period_uq unique (author_id, period)
);
create index if not exists monthly_reports_period_idx on monthly_reports (period desc);
create index if not exists monthly_reports_status_idx on monthly_reports (status);

alter table monthly_reports enable row level security;
