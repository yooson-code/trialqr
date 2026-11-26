/* Skrip bersama untuk semua halaman. */

/**
 * Kompres foto di browser sebelum diunggah: sisi terpanjang maks. 800 px, JPEG kualitas 0.8.
 * Orientasi EXIF ikut diperbaiki dan metadata (GPS) terbuang. Bila gagal, file asli dikembalikan
 * (server tetap mengompres ulang).
 */
window.compressImage = async function (file, maxDim = 800, quality = 0.8) {
  if (!file || !file.type || !file.type.startsWith('image/')) return file;
  const load = async () => {
    if (window.createImageBitmap) {
      try { return await createImageBitmap(file, { imageOrientation: 'from-image' }); } catch (e) {}
    }
    return await new Promise((resolve, reject) => {          // cadangan untuk browser lama
      const url = URL.createObjectURL(file), img = new Image();
      img.onload = () => { URL.revokeObjectURL(url); resolve(img); };
      img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('load')); };
      img.src = url;
    });
  };
  try {
    const src = await load();
    const sw = src.width || src.naturalWidth, sh = src.height || src.naturalHeight;
    const scale = Math.min(1, maxDim / Math.max(sw, sh));
    const w = Math.max(1, Math.round(sw * scale)), h = Math.max(1, Math.round(sh * scale));
    const canvas = document.createElement('canvas');
    canvas.width = w; canvas.height = h;
    const ctx = canvas.getContext('2d');
    ctx.fillStyle = '#fff'; ctx.fillRect(0, 0, w, h);          // PNG transparan -> latar putih
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(src, 0, 0, w, h);
    if (src.close) src.close();
    const blob = await new Promise(r => canvas.toBlob(r, 'image/jpeg', quality));
    if (!blob || (scale === 1 && blob.size >= file.size)) return file; // sudah kecil
    const base = (file.name || 'foto').replace(/\.[^.]+$/, '') || 'foto';
    return new File([blob], base + '.jpg', { type: 'image/jpeg', lastModified: Date.now() });
  } catch (e) {
    return file;
  }
};

/* Getar singkat saat menyentuh tombol/menu (Android; diabaikan bila tidak didukung). */
document.addEventListener('pointerdown', e => {
  if (e.pointerType === 'touch' && navigator.vibrate && e.target.closest('[data-haptic], .btn')) navigator.vibrate(8);
}, { passive: true });
