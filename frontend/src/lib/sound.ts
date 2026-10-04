import successUrl from "@/assets/sounds/success.mp3";
import errorUrl from "@/assets/sounds/error.mp3";

function play(url: string) {
  const audio = new Audio(url);
  audio.play().catch(() => {});
}

export function playSuccessSound() {
  play(successUrl);
}

export function playErrorSound() {
  play(errorUrl);
}
