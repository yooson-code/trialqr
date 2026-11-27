/* Halaman Anggota: modal tambah/ubah, pratinjau umur/kategori, kompres foto, modal QR. */
document.addEventListener('alpine:init', () => {
  Alpine.data('members', () => {
    const D = window.pageData();
    const MEMBERS = D.members || [], CATS = D.categories || [];
    const blank = () => ({ id: '', full_name: '', phone: '', village_id: '', group_id: '', is_active: true, photo_url: null,
      birth_date: '', parent_name: '', parent_phone: '', rfid_uid: '', marital_status: '' });
    const fmt = b => b >= 1048576 ? (b / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(b / 1024)) + ' KB';

    return {
      modal: false, qr: null, preview: null, compressing: false, photoInfo: '', form: blank(),
      villages: D.villages || [], groups: D.groups || [],

      init() {
        // pintasan dari dashboard: /members?add=1
        const q = new URLSearchParams(location.search);
        if (q.get('add') === '1') {
          q.delete('add');
          history.replaceState(history.state, '', location.pathname + (q.toString() ? '?' + q : ''));
          this.$nextTick(() => this.openNew());
        }
      },

      get groupOptions() { return this.groups.filter(g => String(g.village_id) === String(this.form.village_id)); },
      get ageInfo() {
        const b = this.form.birth_date; if (!b) return null;
        const d = new Date(b + 'T00:00:00'), n = new Date();
        let a = n.getFullYear() - d.getFullYear();
        if (n.getMonth() < d.getMonth() || (n.getMonth() === d.getMonth() && n.getDate() < d.getDate())) a--;
        if (isNaN(a) || a < 0 || a > 100) return null;
        const c = CATS.find(c => a >= c.min && a <= c.max);
        return { age: a, label: c ? c.label : '-', key: c ? c.key : '' };
      },
      get isMandiri() { return !!(this.ageInfo && this.ageInfo.key === 'mandiri'); },
      get action() { return this.form.id ? '/members/' + this.form.id : '/members'; },

      openNew() {
        this.form = blank(); this.preview = null; this.photoInfo = ''; this.modal = true;
        this.$nextTick(() => { const el = document.getElementById('f_name'); if (el) el.focus(); });
      },
      openEdit(id) {
        const m = MEMBERS.find(x => x.id === id); if (!m) return;
        this.form = { id: m.id, full_name: m.full_name, phone: m.phone, village_id: m.village_id, group_id: m.group_id,
          is_active: m.is_active, photo_url: m.photo_url, birth_date: m.birth_date ? m.birth_date.slice(0, 10) : '',
          parent_name: m.parent_name || '', parent_phone: m.parent_phone || '', rfid_uid: m.rfid_uid || '',
          marital_status: m.marital_status || '' };
        this.preview = null; this.photoInfo = ''; this.modal = true;
      },
      showQR(id) { this.qr = MEMBERS.find(x => x.id === id) || null; },

      async onPhoto(e) {
        const input = e.target, f = input.files[0];
        this.photoInfo = '';
        if (!f) { this.preview = null; return; }
        this.compressing = true;
        const small = await window.compressImage(f);
        if (small !== f) {
          const dt = new DataTransfer(); dt.items.add(small); input.files = dt.files;   // ganti file yang akan dikirim
          this.photoInfo = 'Dikompres dari ' + fmt(f.size) + ' menjadi ' + fmt(small.size);
        } else {
          this.photoInfo = 'Ukuran ' + fmt(f.size);
        }
        this.preview = URL.createObjectURL(small);
        this.compressing = false;
      },
    };
  });
});
