<template>
  <v-list-item>
    <div class="d-flex align-center">
      <v-icon icon="mdi-information" size="32" class="flash-alert me-2" />
      <v-card class="text-yellow" color="transparent" flat max-width="200" type="info">
        <div class="marquee-wrap">
          <div class="m">
            <span>New Transfer Arrived. Please Check the transfers page for confirmation.</span>
            <span>New Transfer Arrived. Please Check the transfers page for confirmation.</span>
          </div>
        </div>
      </v-card>
    </div>
  </v-list-item>
</template>

<script setup lang="ts"></script>

<style scoped>
.flash-alert {
  animation: flash 1.2s ease-in-out infinite;
}
@keyframes flash {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.3;
  }
}
.marquee-wrap {
  overflow: hidden;
  width: 100%;
}
.m {
  display: flex;
  width: max-content;
  animation: marquee 6s infinite linear;
}
.m span {
  white-space: nowrap;
  font-size: 15px;
  padding-right: 3rem;
}
@keyframes marquee {
  0% {
    transform: translateX(0);
  }
  100% {
    transform: translateX(-50%);
  }
}
</style>
