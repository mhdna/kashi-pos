<template>
  <!-- Our own picture for the empty sale screen: a barcode inside scan-frame
       corners with a laser line across it. Static on purpose (no animation,
       no filters), and drawn in theme colours so it follows light/dark. -->
  <svg
    class="ready-art"
    viewBox="0 0 160 120"
    width="168"
    height="126"
    role="img"
    aria-label="Ready to scan"
  >
    <!-- the card the code is printed on -->
    <rect x="30" y="28" width="100" height="64" rx="9" class="ready-art__card" />
    <!-- barcode: wider than tall, like a real one -->
    <rect
      v-for="bar in bars"
      :key="bar.x"
      :x="bar.x"
      y="38"
      :width="bar.w"
      height="30"
      rx="0.5"
      class="ready-art__bar"
    />
    <!-- the number under it, as on a real label -->
    <text x="46" y="80" textLength="68" lengthAdjust="spacing" class="ready-art__digits">5 901234 123457</text>
    <!-- laser: a faint band with a bright line through it -->
    <rect x="26" y="46" width="108" height="14" rx="7" class="ready-art__glow" />
    <line x1="26" y1="53" x2="134" y2="53" class="ready-art__laser" />
    <!-- scan frame corners, centred on the card -->
    <path d="M20 33 V27 a5 5 0 0 1 5 -5 H31" class="ready-art__corner" />
    <path d="M129 22 H135 a5 5 0 0 1 5 5 V33" class="ready-art__corner" />
    <path d="M140 87 V93 a5 5 0 0 1 -5 5 H129" class="ready-art__corner" />
    <path d="M31 98 H25 a5 5 0 0 1 -5 -5 V87" class="ready-art__corner" />
  </svg>
</template>

<script setup lang="ts">
// Bar widths of a plausible-looking code, scaled to span x = 46..114 so the
// code sits inside the card with the same margins as the digit dashes.
const widths = [3, 1.5, 2, 4, 1.5, 3, 1.5, 2, 1.5, 4, 2, 1.5, 3, 2, 1.5, 4, 1.5, 2, 3, 1.5, 2, 4, 1.5, 3];
const gap = 1.5;
const span = widths.reduce((sum, w) => sum + w, 0) + gap * (widths.length - 1);
const scale = 68 / span;
let x = 46;
const bars = widths.map((w) => {
  const bar = { x: Math.round(x * 100) / 100, w: Math.round(w * scale * 100) / 100 };
  x += (w + gap) * scale;
  return bar;
});
</script>

<style scoped>
.ready-art {
  display: block;
}

.ready-art__card {
  fill: rgb(var(--k-theme-surface));
  stroke: rgba(var(--k-theme-on-surface), 0.12);
  stroke-width: 1.5;
}

.ready-art__bar {
  fill: rgba(var(--k-theme-on-surface), 0.72);
}

.ready-art__digits {
  fill: rgba(var(--k-theme-on-surface), 0.62);
  font-family: Consolas, "Roboto Mono", ui-monospace, monospace;
  font-size: 7.6px;
  font-weight: 700;
}

.ready-art__corner {
  fill: none;
  stroke: rgba(var(--k-theme-on-surface), 0.3);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 3;
}

.ready-art__glow {
  fill: rgb(var(--k-theme-error));
  opacity: 0.14;
}

.ready-art__laser {
  stroke: rgb(var(--k-theme-error));
  stroke-linecap: round;
  stroke-width: 3;
}
</style>
