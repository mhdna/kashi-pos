<template>
  <!-- Our own picture for the empty sale screen: a barcode inside scan-frame
       corners with a laser line across it. Static on purpose (no animation,
       no filters), and drawn in theme colours so it follows light/dark. -->
  <svg
    class="ready-art"
    viewBox="0 0 160 120"
    width="160"
    height="120"
    role="img"
    aria-label="Ready to scan"
  >
    <!-- the soft card behind the code -->
    <rect x="30" y="22" width="100" height="76" rx="10" class="ready-art__card" />
    <!-- barcode -->
    <rect
      v-for="bar in bars"
      :key="bar.x"
      :x="bar.x"
      y="33"
      :width="bar.w"
      height="46"
      rx="0.6"
      class="ready-art__bar"
    />
    <!-- code digits as little dashes -->
    <rect x="46" y="85" width="14" height="3" rx="1.5" class="ready-art__digit" />
    <rect x="64" y="85" width="14" height="3" rx="1.5" class="ready-art__digit" />
    <rect x="82" y="85" width="14" height="3" rx="1.5" class="ready-art__digit" />
    <rect x="100" y="85" width="14" height="3" rx="1.5" class="ready-art__digit" />
    <!-- laser: a faint band with a bright line through it -->
    <rect x="22" y="52" width="116" height="14" rx="7" class="ready-art__glow" />
    <line x1="22" y1="59" x2="138" y2="59" class="ready-art__laser" />
    <!-- scan frame corners -->
    <path d="M14 36 V20 a6 6 0 0 1 6 -6 H36" class="ready-art__corner" />
    <path d="M124 14 H140 a6 6 0 0 1 6 6 V36" class="ready-art__corner" />
    <path d="M146 84 V100 a6 6 0 0 1 -6 6 H124" class="ready-art__corner" />
    <path d="M36 106 H20 a6 6 0 0 1 -6 -6 V84" class="ready-art__corner" />
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

.ready-art__digit {
  fill: rgba(var(--k-theme-on-surface), 0.3);
}

.ready-art__corner {
  fill: none;
  stroke: rgba(var(--k-theme-on-surface), 0.3);
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-width: 5;
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
