<template>
  <k-snackbar
    v-model="visible"
    class="whatsapp-snackbar"
    :timeout="-1"
    location="top"
    variant="flat"
  >
    <div v-if="current" class="whatsapp-toast" role="status" aria-live="polite">
      <k-icon :icon="current.icon" :color="current.color" size="32" />
      <div class="whatsapp-toast__text">
        <div class="whatsapp-toast__title">{{ current.title }}</div>
        <div v-if="current.detail" class="whatsapp-toast__detail">{{ current.detail }}</div>
      </div>
    </div>
  </k-snackbar>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { EventsOn } from "../../wailsjs/runtime/runtime";

// The post-sale WhatsApp message, as seen from the till: "whatsapp_sent"
// when kashi sent it (whatsapp.go), "review_received" when the client
// answered the rating poll (sync_ws.go). Queued like AttendanceToast so two
// at once both show.

interface Notice {
  icon: string;
  color: string;
  title: string;
  detail?: string;
}

const VISIBLE_MS = 5000;
const GAP_MS = 300;

const current = ref<Notice | null>(null);
const visible = ref(false);
const queue: Notice[] = [];
let running = false;
let timer: ReturnType<typeof setTimeout> | undefined;
const offs: Array<() => void> = [];

function showNext() {
  const next = queue.shift();
  if (!next) {
    running = false;
    return;
  }
  running = true;
  current.value = next;
  visible.value = true;
  timer = setTimeout(() => {
    visible.value = false;
    timer = setTimeout(showNext, GAP_MS);
  }, VISIBLE_MS);
}

function enqueue(notice: Notice) {
  queue.push(notice);
  if (!running) showNext();
}

function who(name?: string) {
  return name?.trim() || "A client";
}

onMounted(() => {
  offs.push(
    EventsOn("whatsapp_sent", (m: { client_name?: string; invoice_code?: string }) =>
      enqueue({
        icon: "mdi-whatsapp",
        color: "success",
        title: `WhatsApp sent to ${who(m?.client_name)}`,
        detail: m?.invoice_code ? `Receipt ${m.invoice_code}` : "",
      }),
    ),
    EventsOn("review_received", (r: { client_name?: string; rating?: number }) => {
      const rating = r?.rating ?? 0;
      if (!rating) return;
      if (rating >= 4) enqueue({ icon: "mdi-heart", color: "error", title: `${who(r.client_name)} loved their experience` });
      else if (rating === 3)
        enqueue({ icon: "mdi-emoticon-neutral-outline", color: "warning", title: `${who(r.client_name)} thought their visit was okay` });
      else enqueue({ icon: "mdi-emoticon-sad-outline", color: "grey", title: `${who(r.client_name)} wasn't happy with their visit` });
    }),
  );
});

onUnmounted(() => {
  offs.forEach((off) => off());
  if (timer) clearTimeout(timer);
});
</script>

<style scoped>
.whatsapp-toast {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 310px;
}
.whatsapp-toast__title {
  font-size: 1rem;
  font-weight: 600;
}
.whatsapp-toast__detail {
  font-size: 0.8rem;
  opacity: 0.75;
}
</style>

<style>
/* Solid, unlike AttendanceToast: see-through, the screen under it mixed
   with its text. Unscoped because the snackbar is teleported. */
.whatsapp-snackbar .k-snackbar__wrapper {
  background: rgb(var(--k-theme-surface)) !important;
  color: rgb(var(--k-theme-on-surface)) !important;
  border: 1px solid rgba(var(--k-theme-on-surface), 0.12);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.25) !important;
  /* Sits under home.vue's "Receipt printed" snackbar, which is also at the
     top and usually still up when kashi reports the message sent. */
  margin-top: 76px !important;
}
</style>
