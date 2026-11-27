/* Form laporan musyawarah: daftar kegiatan dinamis. */
document.addEventListener('alpine:init', () => {
  Alpine.data('reportForm', () => {
    const D = window.pageData();
    let seq = 0;
    const mk = o => ({ k: ++seq, name: (o && o.name) || '', status: (o && o.status) || 'berjalan',
      achievement: (o && o.achievement) || '', obstacle: (o && o.obstacle) || '', followup: (o && o.followup) || '' });
    return {
      level: D.level || 'daerah',
      items: ((D.items && D.items.length) ? D.items : [{}]).map(mk),
      add() { if (this.items.length < 40) this.items.push(mk()); },
      remove(i) { this.items.splice(i, 1); if (!this.items.length) this.items.push(mk()); },
    };
  });
});
