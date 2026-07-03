// Local UI feedback only. Every server interaction (render, randomize, gradient
// edits, save) goes through htmx -> the Go handler; this file just keeps range
// readouts, dual-range fills and the gradient swatch in sync as you drag.

function refreshFader(fader) {
  if (!fader) return;
  const out = fader.querySelector('.readout');
  const dual = fader.querySelector('.dual');
  if (dual) {
    const lo = dual.querySelector('.lo');
    const hi = dual.querySelector('.hi');
    const fill = dual.querySelector('.fill');
    const a = Number(lo.value);
    const b = Number(hi.value);
    const min = Number(lo.min);
    const max = Number(lo.max);
    const span = max - min || 1;
    const left = ((Math.min(a, b) - min) / span) * 100;
    const right = ((Math.max(a, b) - min) / span) * 100;
    if (fill) {
      fill.style.left = left + '%';
      fill.style.width = right - left + '%';
    }
    if (out) out.textContent = Math.min(a, b) + ' — ' + Math.max(a, b);
  } else {
    const single = fader.querySelector('input[type="range"]');
    if (out && single) out.textContent = single.value;
  }
}

function refreshGradientBar() {
  const grad = document.querySelector('.gradient');
  if (!grad) return;
  const colors = Array.from(
    grad.querySelectorAll('.stop input[type="color"]'),
  ).map((i) => i.value);
  const bar = grad.querySelector('.bar');
  if (!bar || !colors.length) return;
  bar.style.background =
    colors.length === 1
      ? colors[0]
      : 'linear-gradient(90deg, ' + colors.join(', ') + ')';
}

function initAll(root) {
  (root || document).querySelectorAll('.fader').forEach(refreshFader);
  refreshGradientBar();
}

document.addEventListener('input', (e) => {
  const t = e.target;
  if (t.matches('input[type="range"]')) {
    const dual = t.closest('.dual');
    if (dual) {
      const lo = dual.querySelector('.lo');
      const hi = dual.querySelector('.hi');
      // Keep the two thumbs from crossing: push the other one along.
      if (t.classList.contains('lo') && Number(lo.value) > Number(hi.value)) {
        hi.value = lo.value;
      }
      if (t.classList.contains('hi') && Number(hi.value) < Number(lo.value)) {
        lo.value = hi.value;
      }
    }
    refreshFader(t.closest('.fader'));
  } else if (t.matches('input[type="color"]')) {
    const hex = t.closest('.stop')?.querySelector('.hex');
    if (hex) hex.textContent = t.value.toUpperCase();
    refreshGradientBar();
  }
});

// Re-sync whenever htmx swaps in fresh controls or a new monitor fragment.
document.addEventListener('DOMContentLoaded', () => initAll());
document.body.addEventListener('htmx:load', (e) => initAll(e.target));
document.body.addEventListener('htmx:afterSwap', (e) => initAll(e.target));
