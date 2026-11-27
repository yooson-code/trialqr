/*
 * Navigasi ala aplikasi: tautan dan form ditangani lewat fetch, lalu isi halaman ditukar
 * tanpa memuat ulang halaman (gaya dan skrip tidak diproses ulang). Bila ada yang tidak
 * cocok (unduhan, halaman login, galat), otomatis kembali ke navigasi biasa.
 */
(() => {
  const body = document.body;
  if (!body || !body.hasAttribute('data-shell')) return;       // hanya untuk halaman berkerangka aplikasi

  const calm = window.matchMedia('(prefers-reduced-motion: reduce)');
  let ctrl = null, current = location.pathname + location.search;

  /* ---- bar progres di atas layar ---- */
  const bar = document.createElement('div');
  bar.setAttribute('aria-hidden', 'true');
  bar.style.cssText = 'position:fixed;top:0;left:0;right:0;height:3px;z-index:9999;background:#F2B544;transform-origin:left;transform:scaleX(0);opacity:0;pointer-events:none';
  document.documentElement.appendChild(bar);
  let barTimer;
  const progress = on => {
    clearTimeout(barTimer);
    if (on) {
      bar.style.transition = 'none'; bar.style.opacity = '1'; bar.style.transform = 'scaleX(.05)';
      bar.getBoundingClientRect();
      bar.style.transition = 'transform 6s cubic-bezier(.1,.7,.1,1)'; bar.style.transform = 'scaleX(.9)';
    } else {
      bar.style.transition = 'transform .15s ease-out, opacity .3s ease .15s';
      bar.style.transform = 'scaleX(1)'; bar.style.opacity = '0';
      barTimer = setTimeout(() => { bar.style.transition = 'none'; bar.style.transform = 'scaleX(0)'; }, 500);
    }
  };

  const toast = msg => {
    const el = document.createElement('div');
    el.setAttribute('role', 'status');
    el.className = 'fixed bottom-24 left-4 right-4 z-50 rounded-xl bg-clay-700 px-4 py-3 text-sm text-white shadow-lg sm:left-auto sm:max-w-sm lg:bottom-4';
    el.textContent = msg;
    document.documentElement.appendChild(el);
    setTimeout(() => el.remove(), 4500);
  };

  const sameOrigin = u => u.origin === location.origin;
  const isFile = u => /\/export$|\.(png|csv|xlsx|pdf)$/i.test(u.pathname) || u.searchParams.has('download')
                   || u.pathname.startsWith('/static/') || u.pathname.startsWith('/uploads/');

  function swap(doc, finalUrl, opts) {
    history.replaceState({ y: window.scrollY }, '');
    if (opts.push !== false && finalUrl !== location.href) history.pushState({ y: 0 }, '', finalUrl);
    current = location.pathname + location.search;

    document.title = doc.title;
    const incoming = [...doc.body.childNodes].map(n => document.importNode(n, true));
    if (window.Alpine) { try { Alpine.$data(body).sheet = false; } catch (e) {} }
    body.style.overflow = '';
    body.replaceChildren(...incoming);                         // Alpine menginisialisasi komponen baru otomatis
    window.scrollTo(0, opts.y || 0);
    if (window.hadirEnhance) window.hadirEnhance();
  }

  async function go(url, opts = {}) {
    if (ctrl) ctrl.abort();
    ctrl = new AbortController();
    progress(true);
    const isPost = opts.method === 'POST';
    try {
      const res = await fetch(url, {
        method: opts.method || 'GET', body: opts.body, credentials: 'same-origin',
        signal: ctrl.signal, headers: { 'X-Requested-With': 'hadir-nav' }, redirect: 'follow',
      });
      const type = res.headers.get('content-type') || '';
      if (!type.includes('text/html')) {                       // mis. unduhan: serahkan ke browser
        if (isPost) throw new Error('bukan halaman');
        location.href = url; return;
      }
      const doc = new DOMParser().parseFromString(await res.text(), 'text/html');
      if (!doc.body.hasAttribute('data-shell')) { location.href = res.url; return; } // mis. halaman login
      swap(doc, res.url, opts);
    } catch (e) {
      if (e.name === 'AbortError') return;
      if (isPost) toast('Koneksi bermasalah. Periksa jaringan lalu coba lagi.');
      else location.href = typeof url === 'string' ? url : location.href;
    } finally {
      progress(false);
    }
  }

  document.addEventListener('click', e => {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = e.target.closest('a[href]');
    if (!a || (a.target && a.target !== '_self') || a.hasAttribute('download') || a.hasAttribute('data-no-boost')) return;
    const u = new URL(a.href, location.href);
    if (!sameOrigin(u) || isFile(u)) return;
    if (u.pathname === location.pathname && u.search === location.search && u.hash) return; // lompat ke #bagian
    e.preventDefault();
    go(u.href);
  });

  document.addEventListener('submit', e => {
    if (e.defaultPrevented) return;                             // mis. dibatalkan oleh confirm()
    const f = e.target, sub = e.submitter;
    if (f.hasAttribute('data-no-boost') || (f.target && f.target !== '_self')) return;
    const method = ((sub && sub.getAttribute('formmethod')) || f.method || 'get').toUpperCase();
    const action = new URL((sub && sub.getAttribute('formaction')) || f.action, location.href);
    if (!sameOrigin(action) || isFile(action)) return;
    e.preventDefault();
    let fd;
    try { fd = new FormData(f, sub); } catch (err) { fd = new FormData(f); if (sub && sub.name) fd.append(sub.name, sub.value); }
    if (method === 'GET') {
      action.search = new URLSearchParams([...fd].filter(([, v]) => typeof v === 'string')).toString();
      go(action.href);
    } else {
      go(action.href, { method: 'POST', body: fd });
    }
  });

  window.addEventListener('popstate', e => {
    const now = location.pathname + location.search;
    if (now === current) return;                                // hanya berubah #hash
    go(location.href, { push: false, y: (e.state && e.state.y) || 0 });
  });
})();
