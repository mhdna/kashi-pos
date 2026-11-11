// @vitest-environment happy-dom
//
// The balance page's "Coming up" card: folded to one button with a count by
// default, the list only after tapping it.
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import ui from "@/ui";

const push = vi.fn();
vi.mock("vue-router", () => ({ useRouter: () => ({ push }) }));
vi.mock("../../../wailsjs/go/main/App", () => ({ ListCalendarEvents: vi.fn(), GetCalendarSettings: vi.fn() }));
vi.mock("../../../wailsjs/runtime/runtime", () => ({ EventsOn: vi.fn(() => () => {}) }));

import { GetCalendarSettings, ListCalendarEvents } from "../../../wailsjs/go/main/App";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import EventsCard from "./EventsCard.vue";

// An empty catalogue, so only the events a test gives are on the calendar.
const SETTINGS = { businessType: "clothing", hiddenEvents: [], catalog: JSON.stringify({ events: [], businessTypes: {}, lists: {} }) };
const ev = (id: number, name: string, startDate: string, endDate = startDate) => ({ id, name, icon: "", color: "", startDate, endDate });

let wrapper: VueWrapper<any> | null = null;
async function mountCard(events: any[]) {
  vi.mocked(ListCalendarEvents).mockResolvedValue(events as any);
  vi.mocked(GetCalendarSettings).mockResolvedValue(SETTINGS as any);
  wrapper = mount(EventsCard, { global: { plugins: [ui] }, attachTo: document.body });
  await flushPromises();
  return wrapper;
}
const toggle = () => wrapper!.find(".events-card-toggle");
const count = () => wrapper!.find(".events-card-count").text();
const next = () => wrapper!.find(".events-card-next");
const names = () => wrapper!.findAll(".events-card-name").map((n) => n.text());

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 7, 10, 0)); // 7 Nov 2026
  vi.clearAllMocks();
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = null;
  vi.useRealTimers();
});

describe("EventsCard", () => {
  it("starts folded: only the button, no list and no calendar link", async () => {
    await mountCard([ev(1, "Black Friday", "2026-11-27")]);
    expect(toggle().exists()).toBe(true);
    expect(toggle().attributes("aria-expanded")).toBe("false");
    expect(wrapper!.find(".events-card-list").exists()).toBe(false);
    expect(wrapper!.find(".events-card-calendar").exists()).toBe(false);
    expect(names()).toEqual([]);
  });

  it("counts what is on now or starts within 30 days", async () => {
    await mountCard([
      ev(1, "Running sale", "2026-11-01", "2026-11-10"), // on now
      ev(2, "Black Friday", "2026-11-27"),
      ev(3, "Day 30", "2026-12-07"), // 30 days out
      ev(4, "Christmas", "2026-12-25"), // later: not counted
      ev(5, "Past", "2026-10-01"), // over: not counted
    ]);
    expect(count()).toBe("3 events in the next 30 days");
  });

  it("says one event in the singular", async () => {
    await mountCard([ev(1, "Black Friday", "2026-11-27")]);
    expect(count()).toBe("1 event in the next 30 days");
  });

  it("says when nothing is coming", async () => {
    await mountCard([ev(4, "Christmas", "2026-12-25")]);
    expect(count()).toBe("No events in the next 30 days");
  });

  it("opens the list on tap, with the calendar link, and folds again", async () => {
    await mountCard([ev(1, "Black Friday", "2026-11-27"), ev(4, "Christmas", "2026-12-25")]);
    await toggle().trigger("click");
    expect(toggle().attributes("aria-expanded")).toBe("true");
    expect(names()).toEqual(["Black Friday", "Christmas"]); // the list still looks 90 days ahead
    expect(wrapper!.text()).toContain("In 20 days");

    await wrapper!.find(".events-card-calendar").trigger("click");
    expect(push).toHaveBeenCalledWith("/events");

    await toggle().trigger("click");
    expect(wrapper!.find(".events-card-list").exists()).toBe(false);
  });

  it("is a bar across the page, as wide as the cards under it", async () => {
    await mountCard([ev(1, "Black Friday", "2026-11-27")]);
    const card = wrapper!.find(".events-card");
    // The whole bar is the button, not a small chip.
    expect(toggle().element.tagName).toBe("BUTTON");
    expect(card.element.firstElementChild!.contains(toggle().element)).toBe(true);
    // Vitest doesn't load component CSS: the rules are read from the source.
    const css = readFileSync(resolve(process.cwd(), "src/components/shifts/EventsCard.vue"), "utf8");
    expect(css).toMatch(/\.events-card \{[^}]*width: 100%/);
    expect(css).toMatch(/\.events-card \{[^}]*border: 1px solid/);
    expect(css).toMatch(/\.events-card-toggle \{[^}]*flex: 1 1 auto/);
  });

  it("names the next event on the folded bar, and hides it when open", async () => {
    await mountCard([ev(4, "Christmas", "2026-12-25"), ev(1, "Black Friday", "2026-11-27")]);
    expect(next().text()).toBe("Next: Black Friday · In 20 days");
    await toggle().trigger("click");
    expect(next().exists()).toBe(false);
  });

  it("says an event on now is now, and names nothing when nothing is coming", async () => {
    await mountCard([ev(1, "Running sale", "2026-11-01", "2026-11-10")]);
    expect(next().text()).toBe("Next: Running sale · Now");
    wrapper!.unmount();
    await mountCard([ev(4, "Christmas", "2026-12-25")]);
    expect(next().exists()).toBe(false);
  });

  it("updates the count when central sends new events", async () => {
    await mountCard([]);
    expect(count()).toBe("No events in the next 30 days");
    const onEvents = vi.mocked(EventsOn).mock.calls.find(([name]) => name === "sync_applied:calendar_events")![1];
    vi.mocked(ListCalendarEvents).mockResolvedValue([ev(1, "Black Friday", "2026-11-27")] as any);
    onEvents();
    await flushPromises();
    expect(count()).toBe("1 event in the next 30 days");
  });
});
