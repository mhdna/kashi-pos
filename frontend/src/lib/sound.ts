import successUrl from "@/assets/sounds/success.mp3";
import errorUrl from "@/assets/sounds/error.mp3";
import warningUrl from "@/assets/sounds/warning.mp3";
import clickUrl from "@/assets/sounds/click.mp3";
import dtmf0 from "@/assets/sounds/dtmf/0.mp3";
import dtmf1 from "@/assets/sounds/dtmf/1.mp3";
import dtmf2 from "@/assets/sounds/dtmf/2.mp3";
import dtmf3 from "@/assets/sounds/dtmf/3.mp3";
import dtmf4 from "@/assets/sounds/dtmf/4.mp3";
import dtmf5 from "@/assets/sounds/dtmf/5.mp3";
import dtmf6 from "@/assets/sounds/dtmf/6.mp3";
import dtmf7 from "@/assets/sounds/dtmf/7.mp3";
import dtmf8 from "@/assets/sounds/dtmf/8.mp3";
import dtmf9 from "@/assets/sounds/dtmf/9.mp3";
import dtmfStar from "@/assets/sounds/dtmf/star.mp3";
import dtmfPound from "@/assets/sounds/dtmf/pound.mp3";

// Real DTMF (dual-tone multi-frequency) tones, synthesized at the standard
// telephone-keypad frequencies - not a recording, so no licensing to track.
const dtmfTones: Record<string, string> = {
  "0": dtmf0,
  "1": dtmf1,
  "2": dtmf2,
  "3": dtmf3,
  "4": dtmf4,
  "5": dtmf5,
  "6": dtmf6,
  "7": dtmf7,
  "8": dtmf8,
  "9": dtmf9,
  "*": dtmfStar,
  "#": dtmfPound,
};

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

export function playWarningSound() {
  play(warningUrl);
}

export function playClickSound() {
  play(clickUrl);
}

// digit: "0"-"9", "*", or "#". Falls back to the plain click for anything
// else (e.g. the numpad's clear/backspace keys).
export function playDtmfSound(digit: string) {
  const url = dtmfTones[digit];
  play(url ?? clickUrl);
}
