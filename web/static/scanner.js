/* Halaman Scanner: kamera QR, kartu RFID, absen manual. */
document.addEventListener('alpine:init', () => {
  Alpine.data('scanner', () => {
    const D = window.pageData();
    const SCHEDULES = D.schedules || [];
    const VIA = { qr: 'QR', rfid: 'RFID', manual: 'Manual' };
    const QR_LIB = 'https://cdn.jsdelivr.net/npm/html5-qrcode@2.3.8/html5-qrcode.min.js';
    const loadQR = () => window.Html5Qrcode ? Promise.resolve() : new Promise((ok, fail) => {
      const s = document.createElement('script');
      s.src = QR_LIB; s.onload = ok; s.onerror = () => fail(new Error('qr-lib'));
      document.head.appendChild(s);
    });

    return {
      scheduleId: '', mode: 'qr', running: false, muted: false, err: '',
      cameras: [], camIdx: 0, qr: null, ac: null,
      flash: null, result: null, log: [], count: 0, last: { key: '', at: 0 }, lock: false,
      rfidBuf: '', rfidFocused: false, manualQ: '', manualRes: [], searching: false, _off: [],

      get scope() {
        const s = SCHEDULES.find(x => String(x.id) === String(this.scheduleId));
        if (!s || s.level === 'daerah') return { restricted: false, text: 'Terbuka untuk semua anggota daerah.' };
        return { restricted: true, text: 'Khusus anggota ' + s.scope + '.' };
      },

      init() {
        const saved = localStorage.getItem('hadir.schedule');
        const ids = SCHEDULES.map(s => String(s.id));
        this.scheduleId = ids.includes(saved) ? saved : (ids[0] || '');
        this.muted = localStorage.getItem('hadir.muted') === '1';
        this.$watch('scheduleId', v => localStorage.setItem('hadir.schedule', v));
        this.$watch('muted', v => localStorage.setItem('hadir.muted', v ? '1' : '0'));
        loadQR().catch(() => {});                                  // siapkan pustaka kamera lebih awal

        // Reader RFID mengetik seperti keyboard: pastikan input selalu menerima fokus.
        const refocus = () => {
          if (this.mode === 'rfid' && this.$refs.rfid && !['INPUT', 'SELECT', 'TEXTAREA'].includes(document.activeElement.tagName)) this.$refs.rfid.focus();
        };
        const onClick = e => { if (!e.target.closest('input,select,textarea,button,a')) refocus(); };
        document.addEventListener('click', onClick);
        window.addEventListener('keydown', refocus);
        this._off.push(() => document.removeEventListener('click', onClick), () => window.removeEventListener('keydown', refocus));
        const r = this.$refs.rfid;
        if (r) {
          const f = () => (this.rfidFocused = true), b = () => (this.rfidFocused = false);
          r.addEventListener('focus', f); r.addEventListener('blur', b);
        }
      },

      destroy() {                                                  // dipanggil Alpine saat pindah halaman
        this._off.forEach(fn => fn());
        this.stop();
      },

      async setMode(m) {
        if (m === this.mode) return;
        if (m === 'rfid') await this.stop();
        this.mode = m;
        this.ensureAudio();
        if (m === 'rfid') this.$nextTick(() => this.$refs.rfid.focus());
      },

      ensureAudio() {
        this.ac = this.ac || new (window.AudioContext || window.webkitAudioContext)();
        if (this.ac.state === 'suspended') this.ac.resume();
      },

      /* ---------- Kamera QR ---------- */
      async start(cameraId) {
        this.err = '';
        try {
          this.ensureAudio();                                      // harus dibuat dari klik pengguna
          await loadQR();
          this.qr = this.qr || new Html5Qrcode('reader', { formatsToSupport: [Html5QrcodeSupportedFormats.QR_CODE], verbose: false });
          await this.qr.start(
            cameraId ? { deviceId: { exact: cameraId } } : { facingMode: 'environment' },
            { fps: 12, aspectRatio: 1, qrbox: (w, h) => { const s = Math.floor(Math.min(w, h) * 0.68); return { width: s, height: s }; } },
            txt => this.send({ token: txt }, 'qr:' + txt, 'qr'), () => {}
          );
          this.running = true;
          if (!this.cameras.length) { try { this.cameras = await Html5Qrcode.getCameras(); } catch (e) {} }
        } catch (e) {
          this.running = false;
          this.err = e && e.message === 'qr-lib' ? 'Pustaka pemindai gagal dimuat. Periksa koneksi internet.'
            : (location.protocol !== 'https:' && location.hostname !== 'localhost')
              ? 'Browser hanya mengizinkan kamera lewat HTTPS atau localhost.'
              : 'Izinkan akses kamera di browser, lalu coba lagi.';
        }
      },
      async stop() {
        try { if (this.qr && this.qr.isScanning) await this.qr.stop(); } catch (e) {}
        this.running = false;
      },
      async switchCamera() {
        if (this.cameras.length < 2) return;
        this.camIdx = (this.camIdx + 1) % this.cameras.length;
        await this.stop();
        await this.start(this.cameras[this.camIdx].id);
      },

      /* ---------- RFID & manual ---------- */
      submitRFID() {
        const uid = this.rfidBuf.trim();
        this.rfidBuf = '';
        if (uid) this.send({ rfid_uid: uid }, 'rfid:' + uid.toUpperCase(), 'rfid');
      },
      async searchManual() {
        const q = this.manualQ.trim();
        if (q.length < 2) { this.manualRes = []; return; }
        this.searching = true;
        try {
          const r = await fetch('/api/v1/members/search?q=' + encodeURIComponent(q), { credentials: 'same-origin' });
          if (r.status === 401) { location.href = '/login'; return; }
          this.manualRes = (await r.json()).members || [];
        } catch (e) { this.manualRes = []; }
        this.searching = false;
      },
      async markManual(m) {
        await this.send({ member_id: m.id }, 'manual:' + m.id, 'manual', true);
        this.manualQ = ''; this.manualRes = [];
      },

      /* ---------- kirim ke API ---------- */
      async send(payload, key, via, force) {
        const now = Date.now();
        if (this.lock) return;
        if (!force && key === this.last.key && now - this.last.at < 3500) return; // kartu/QR yang sama masih menempel
        this.last = { key, at: now };
        this.lock = true;
        try {
          const res = await fetch('/api/v1/scan', {
            method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ...payload, schedule_id: Number(this.scheduleId) }),
          });
          if (res.status === 401) { location.href = '/login'; return; }
          const d = await res.json().catch(() => ({ status: 'error', message: 'Respons server tidak terbaca' }));
          this.show(d, via);
        } catch (e) {
          this.show({ status: 'error', message: 'Tidak ada koneksi ke server' }, via);
        } finally {
          setTimeout(() => { this.lock = false; }, 1200);
        }
      },

      show(d, via) {
        const tone = d.status === 'success' ? 'ok' : d.status === 'duplicate' ? 'dup' : 'bad';
        const title = { ok: 'Hadir', dup: 'Sudah absen',
          bad: d.reason === 'scope' ? 'Di luar cakupan' : d.reason === 'inactive' ? 'Anggota nonaktif'
             : d.status === 'invalid' ? (via === 'rfid' ? 'Kartu tidak dikenal' : 'QR tidak valid') : 'Gagal' }[tone];
        const detail = tone === 'ok' ? 'Tercatat pukul ' + d.scanned_at + ' WIB'
                     : tone === 'dup' ? 'Sudah absen hari ini pukul ' + d.scanned_at + ' WIB' : d.message;
        this.result = { tone, title, detail, member: d.member || null };
        this.log.unshift({ key: Date.now() + Math.random(), tone, via: VIA[via] || '', name: d.member ? d.member.name : title,
          time: new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', second: '2-digit' }) });
        this.log = this.log.slice(0, 8);
        if (tone === 'ok') this.count++;

        this.flash = tone;
        clearTimeout(this._ft);
        this._ft = setTimeout(() => { this.flash = null; }, 1000);
        this.beep(tone);
        if (navigator.vibrate) navigator.vibrate(tone === 'ok' ? 80 : tone === 'dup' ? [60, 50, 60] : 300);
      },

      beep(tone) {
        if (this.muted || !this.ac) return;
        const seq = { ok: [[880, .08], [1320, .14]], dup: [[520, .1], [520, .1]], bad: [[170, .35]] }[tone] || [];
        let t = this.ac.currentTime;
        seq.forEach(([f, d]) => {
          const o = this.ac.createOscillator(), g = this.ac.createGain();
          o.type = tone === 'bad' ? 'sawtooth' : 'sine';
          o.frequency.value = f;
          g.gain.setValueAtTime(0.0001, t);
          g.gain.exponentialRampToValueAtTime(0.28, t + 0.01);
          g.gain.exponentialRampToValueAtTime(0.0001, t + d);
          o.connect(g); g.connect(this.ac.destination);
          o.start(t); o.stop(t + d + 0.02);
          t += d + 0.05;
        });
      },
    };
  });
});
