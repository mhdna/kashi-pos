<template>
  <div class="floating-icons">
    <div
      v-for="(icon, i) in clothes"
      :key="i"
      class="icon"
      :style="{
        left: icon.x + '%',
        top: icon.y + '%',
        fontSize: icon.size + 'px',
        opacity: icon.opacity,
        transform: `rotate(${icon.rotation}deg)`,
        animationDelay: icon.delay + 's',
        animationDuration: icon.duration + 's',
      }"
    >
      <v-icon>{{ "mdi-" + icon.name }}</v-icon>
    </div>
  </div>
</template>

<script setup>
const clothes = [
  {
    name: "tshirt-crew",
    x: 8,
    y: 12,
    size: 47,
    opacity: 0.04,
    rotation: -5,
    delay: 0,
    duration: 5,
  },
  {
    name: "hanger",
    x: 85,
    y: 8,
    size: 60,
    opacity: 0.06,
    rotation: 10,
    delay: 1.5,
    duration: 3,
  },
  {
    name: "shoe-sneaker",
    x: 75,
    y: 75,
    size: 64,
    opacity: 0.05,
    rotation: -15,
    delay: 0.8,
    duration: 5,
  },
  {
    name: "hat-fedora",
    x: 15,
    y: 70,
    size: 49,
    opacity: 0.04,
    rotation: 8,
    delay: 2.2,
    duration: 5.5,
  },
  {
    name: "tie",
    x: 50,
    y: 15,
    size: 37,
    opacity: 0.05,
    rotation: -8,
    delay: 0.3,
    duration: 9,
  },
  {
    name: "bag-personal",
    x: 90,
    y: 50,
    size: 43,
    opacity: 0.03,
    rotation: 12,
    delay: 3,
    duration: 5.5,
  },
  {
    name: "sunglasses",
    x: 10,
    y: 45,
    size: 41,
    opacity: 0.06,
    rotation: -12,
    delay: 1,
    duration: 6.5,
  },
  {
    name: "watch",
    x: 65,
    y: 88,
    size: 39,
    opacity: 0.05,
    rotation: 5,
    delay: 2.5,
    duration: 5,
  },
  {
    name: "necklace",
    x: 40,
    y: 60,
    size: 34,
    opacity: 0.07,
    rotation: -20,
    delay: 0.5,
    duration: 4.5,
  },
  {
    name: "coat-rack",
    x: 92,
    y: 85,
    size: 42,
    opacity: 0.03,
    rotation: 3,
    delay: 4,
    duration: 7,
  },
  {
    name: "tshirt-crew-outline",
    x: 20,
    y: 88,
    size: 30,
    opacity: 0.04,
    rotation: 18,
    delay: 1.8,
    duration: 4.5,
  },
  {
    name: "bow-tie",
    x: 55,
    y: 45,
    size: 40,
    opacity: 0.08,
    rotation: -5,
    delay: 3.5,
    duration: 5.5,
  },
  {
    name: "purse",
    x: 30,
    y: 30,
    size: 28,
    opacity: 0.04,
    rotation: 15,
    delay: 2,
    duration: 4,
  },
  {
    name: "umbrella",
    x: 78,
    y: 22,
    size: 35,
    opacity: 0.03,
    rotation: -25,
    delay: 0.2,
    duration: 3.5,
  },
  {
    name: "ring",
    x: 45,
    y: 80,
    size: 18,
    opacity: 0.06,
    rotation: 20,
    delay: 4.5,
    duration: 3,
  },
  {
    name: "account-tie",
    x: 5,
    y: 85,
    size: 40,
    opacity: 0.03,
    rotation: -3,
    delay: 1.2,
    duration: 4,
  },
  {
    name: "glasses",
    x: 70,
    y: 10,
    size: 28,
    opacity: 0.05,
    rotation: 7,
    delay: 2.8,
    duration: 3.5,
  },
];
</script>

<style scoped>
.floating-icons {
  position: fixed;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
  z-index: 0;
}
.icon {
  position: absolute;
  animation: float 6s ease-in-out infinite;
  will-change: transform;
}
@keyframes float {
  0%,
  100% {
    transform: translateY(0px);
  }
  50% {
    transform: translateY(-18px);
  }
}
</style>
